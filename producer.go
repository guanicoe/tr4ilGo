package main

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/vbauerster/mpb"
	"github.com/vbauerster/mpb/decor"
)

type jobData struct {
	WorkQueue      chan workRequest
	Result         chan workOutput
	paramPointer   *JobParam
	unscrapedFiles chan string
	unscrapedLen   int
	fileSent       []string
	fileSentLen    int
	fileRecv       []string
	fileRecvLen    int
	continueProd   bool
	err            error
}
type dirStruct struct {
	name   string //Name folder in collection
	parent string //Name fo collection
	path   string // path to name
	file   string // name file in name folder
	leakID int
}
type fileStr struct {
	name  string
	lines int
}
type workRequest struct {
	Line string
	Job  dirStruct
	DB   *sql.DB
	WG   *sync.WaitGroup
}

type workOutput struct {
	Work  workRequest
	Error error
}

/*
Core function producerm it creats the context, and sends the jobs to workers.
It checks what urls were visited, and creats a struct to keep all the data in one place.
The context is sent to the workers in order to stop them when the work is done.
param:
	- fv pointer
	- return output in JsonOutput and error
*/
func startProducer(param *JobParam) {

	//Declare context for current job
	var (
		ctx    context.Context
		cancel context.CancelFunc
	)
	ctx, cancel = context.WithTimeout(context.Background(), time.Second*time.Duration(20000))
	// local struct contaning all the data. only pointers are sent across
	s := jobData{
		WorkQueue: make(chan workRequest, 1000),
		Result:    make(chan workOutput, 1000),

		paramPointer: param,

		unscrapedFiles: make(chan string),
		unscrapedLen:   1,
		fileSentLen:    0,
		fileRecvLen:    0,
		continueProd:   true,
		err:            nil,
	}

	var wg sync.WaitGroup //Local wait group to wait for the main loop that is sent as a go routing

	startDispatcher(ctx, numWorkers, &s) //Calling the dispatcher function that will start the workers and distribute the work

	log.Debug("Sending initial job ")
	// start := time.Now() //get time as start to give a few seconds wait before timing out if nothing is received from workers

	sendWork(ctx, s.paramPointer.JobList, &s) // starting a goroutiing of the sendWork
	// go sendWork(ctx, s.paramPointer.JobList, &s) // starting a goroutiing of the sendWork
	wg.Add(1)

	//main loop as gorouting to listen and clean worker result and send new urls for scraping
	go func(ctx context.Context, wg *sync.WaitGroup) {
		p := mpb.New(mpb.WithWidth(64))

		total := s.fileSentLen
		name := "Single Bar:"
		// adding a single bar, which will inherit container's width
		bar := p.Add(int64(total),
			// progress bar filler with customized style
			mpb.NewBarFiller("╢▌▌░╟"),
			mpb.PrependDecorators(
				// display our name with one space on the right
				decor.Name(name, decor.WC{W: len(name) + 1, C: decor.DidentRight}),
				// replace ETA decorator with "done" message, OnComplete event
				decor.OnComplete(
					decor.AverageETA(decor.ET_STYLE_GO, decor.WC{W: 4}), "done",
				),
			),
			mpb.AppendDecorators(decor.Percentage()),
		)
		defer wg.Done()
		for {

			//switch condition to verify that we have not hit a wall
			switch {
			case s.fileRecvLen == s.fileSentLen:
				// if we received the same number as we sent and that we waited a bit to make sure the workers are not working, we exit
				return
			default:
				//If we havn't hit a stop condition, we go and listen for workers
				select {
				case <-ctx.Done(): // in case the context is canceled
					return
				case r := <-s.Result: // listening on result channel for workers' output
					s.fileRecv = append(s.fileRecv, fmt.Sprint(r.Work)) //We add turl the worker scraped to our receive slice
					s.fileRecvLen++                                     // and we increment the length
					msg := fmt.Sprintf("Sent %v | Received %v ", s.fileSentLen, s.fileRecvLen)
					fmt.Printf("\r%s", msg) // lazy printing of progression on terminal
					bar.Increment()
					processResult(ctx, r, &s) // we send result to the process function

				case <-time.After(2 * time.Second): // we loop every 2 seconds in order not to block on Result
				}
			}
		}
		p.Wait()
	}(ctx, &wg)
	wg.Wait()
	cancel() //We cancel the context when the go loop returns

	// TODO: check if necessary. Quick for loop to purge the work buffered queue
	for len(s.WorkQueue) > 0 {
		<-s.WorkQueue
	}

}

/*
Dispatcher function. It first creats a go routing for each worker and sends the appropriate
data. In then sents an embedded go routine which listens for available workers.
Once it gets one, it it grabs a work from the work queue and sends it to a worker
*/
func startDispatcher(ctx context.Context, n int, s *jobData) {

	// First, initialize the channel we are going to put the workers' work channels into.
	PugsQueue := make(chan chan workRequest, n)
	// Now, create all of our workers.
	for i := 0; i < n; i++ {
		log.Info(fmt.Sprintf("\rStarting worker %v/%v", i+1, n))
		worker := workerNew(ctx, i+1, s.paramPointer.DB, s.paramPointer.Mutex, PugsQueue, s.Result)
		worker.Start()
	}
	// fmt.Printf("\n")

	go func(ctx context.Context, s *jobData) {
		for {
			select {
			case <-ctx.Done():
				return
			case work := <-s.WorkQueue:

				go func(ctx context.Context) {
					select {
					case <-ctx.Done():
						return
					default:
						worker := <-PugsQueue
						worker <- work
					}

				}(ctx)

			}
		}
	}(ctx, s)
	return
}

/*
function ran in go routing from producer. This function first sends the initial
target url, it then listens for the processresult function for unscraped urls
and sends them to the workaueue buffered channel
param:
	- firstURL string initial url
	- ctx
	- s pointer to producer currated results
*/
func sendWork(ctx context.Context, jobs []dirStruct, s *jobData) {

	sendToPugs := func(l dirStruct) {
		s.unscrapedLen--
		work := workRequest{Job: l}
		s.WorkQueue <- work
	}
	for _, j := range jobs {

		sendToPugs(j)
		s.fileSentLen++
	}

	// for {
	// 	select {
	// 	case <-ctx.Done():
	// 		return
	// 	case l := <-s.unscrapedURL:

	// 		sendToPugs(l)
	// 		s.scrapedSent = append(s.scrapedSent, l)
	// 		s.scrapedSentLen++

	// 	}

	// }

}

func processResult(ctx context.Context, r workOutput, s *jobData) {
	err := ChangeStatus(s.paramPointer.DB, 2, r.Work.Job.leakID)
	CheckErr(err, "Error", "Could not change status in DB")

}
