package main

import (
	"flag"

	"bitbucket.org/grgbrn/localfm"
)

func main() {

	recoverPtr := flag.Bool("recover", false, "Try to recover from a checkpoint file")
	delayPtr := flag.Int("delay", 5, "Delay in seconds between API calls")
	limitPtr := flag.Int("limit", 0, "Limit number of API calls")

	localfm.Main(*delayPtr, *limitPtr, *recoverPtr)
}
