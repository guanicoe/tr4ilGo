# Tr4ilGo

## Introduction
Tr4ilGo is a small piece of code that aims to process dataleaks, specifically email:password files that can be found on hacker forums, into a sqlite database. 
These dataleaks can easilly be a few Gbytes and thus using Python and linearly processing these files will take forever. Tr4ilGo tries to use goroutines to parallise this process. It still takes time, but it is still faster than Python.

This software is still in alpha. It works, but I'm still trying to make it a bit more user friendly. This is where you come in. If you want to help out, you can:

- give me some feedback on what you want tr4ilGo to do
- push some helpfull modification to improve the software

## How to use?
This software being in alpha, there is no interface, nor command line interactions. If you want to change something, you'll need to modify the code. This will change when a beta version is reached.

### Install Go
First, you'll need to install [Golang](https://golang.org/). I'm working with `go version go1.15.8 linux/amd64`. But later versions should work. 

There are a few dependencies that you will also need

    go get github.com/vbauerster/mpb
    go get github.com/sirupsen/logrus
    go get github.com/evilsocket/islazy/tui
    go get github.com/mattn/go-sqlite3

### Editing the source code for you application
In the `main.go` file, there are a few constants that you will want to modify. Specifically 
    
```go
cwd = "/media/parrot/HASH DB"
parent = "Collection 1"
```

**IMPORTANT** : for the moment, the file structure for where the email:pwd files are is important. It needs to follow the following structure. 
`cwd` is the name of the folder where you store your file leaks. For me it's an external HDD named `HASH DB`. Then in there you should have a folder with the collection of leaks `Parent`. For the moment, this folder is hardcoded to be named `Collection 1`, but it can be anything. 
Finally, in these collection, there should be a list of directories in which the text files are. 

```
media/parrot/HASH DB
└── Parent
    ├── name1
    │   ├── 0.txt
    │   ├── 1.txt
    │   └── 2.txt
    └── name2
        ├── 0.txt
        ├── 1.txt
        └── 2.txt
```

A text file should be in the following format

```
email@domain.com:password
```
In case the seperator is something else, such as `;` as it can be in some file, you can either change it in the txt file, or you can add a condition in `worker.go`

```go
switch {
		case strings.Contains(line, ":"):
			seperator = ":"
		case strings.Contains(line, ";"):
			seperator = ";"
		}
```
### Compiling from source
Now you should be able to compile from source. you can clone the repo and build it

    gh repo clone guanicoe/tr4ilGo
    go build -o tr4ilGo

### Run the program
If everything goes to correctly, you can run the program with `sudo`

    sudo ./tr4ilGo
    
## TODO
- I'll add some flexibility and some sort of menu so the program can be used in cli. 
- I will also change the database structure as it can be obtimised. 
- Might also add some tools to actually interact with the database such as showing stats, or filtering tools to generate clean files. 
