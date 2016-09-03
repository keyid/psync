package commands

import (
	"os"
	"log"
	"path/filepath"
	"github.com/eugene-eeo/psync/lib"
	"github.com/parnurzeal/gorequest"
)

type request struct {
	url      string
	checksum lib.Checksum
}

type response struct {
	checksum lib.Checksum
	block    *lib.Block
}

func fetchBlock(requests <-chan *request, responses chan<- *response, done chan<- bool) {
	for req := range requests {
		resp, body, errors := gorequest.New().Get(req.url).EndBytes()
		if len(errors) != 0 || resp.StatusCode != 200 {
			log.Fatal("cannot fetch block: ", req.checksum)
		}
		responses <- &response{
			checksum: req.checksum,
			block:    lib.NewBlock(body),
		}
	}
	done <- true
}

func writeBlocks(responses <-chan *response, done chan<- bool) {
	root := lib.BlocksDir()
	for b := range responses {
		if b.checksum != b.block.Checksum {
			log.Fatal("invalid block: ", b.checksum)
		}
		f, err := os.Create(filepath.Join(root, string(b.checksum)))
		CheckError(err)
		b.block.WriteTo(f)
		f.Close()
	}
	done <- true
}

func Get(addr string, hashlist_path string, force bool) {
	f, err := os.Open(hashlist_path)
	CheckError(err)
	hashlist := lib.NewHashList(f)

	requests := make(chan *request, 10)
	responses := make(chan *response, 10)
	fetch_done := make(chan bool)
	write_done := make(chan bool)

	var missing []lib.Checksum = *hashlist
	if !force {
		missing = hashlist.Missing()
	}

	for i := 0; i < 10; i++ {
		go fetchBlock(requests, responses, fetch_done)
	}

	go func() {
		for _, checksum := range missing {
			requests <- &request{
				url: addr + "/" + string(checksum),
				checksum: checksum,
			}
		}
		close(requests)
	}()

	go writeBlocks(responses, write_done)
	for i := 0; i < 10; i++ {
		<-fetch_done
	}
	close(responses)
	<-write_done
}
