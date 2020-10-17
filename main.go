package main

import (
	"github.com/tj8000rpm/tjsip/sip"
	"log"
)

func main() {
	recieveBufSizeB = 9000
	log.SetOutput(os.Stdout)
	done := tjsip.Serve("0.0.0.0", 5060)
	<-done
}
