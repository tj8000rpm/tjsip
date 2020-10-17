package main

import (
	//"github.com/tj8000rpm/tjsip/sip"
	"log"
	"os"
	"sip/sip"
)

func main() {
	sip.RecieveBufSizeB = 9000
	log.SetOutput(os.Stdout)
	sip.ListenAndServe("0.0.0.0:5060", nil)
}
