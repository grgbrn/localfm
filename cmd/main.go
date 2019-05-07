package main

import (
	"flag"

	"bitbucket.org/grgbrn/localfm"
)

func main() {

	delayPtr := flag.Int("delay", 5, "Delay in seconds between API calls")
	limitPtr := flag.Int("limit", 0, "Limit number of API calls")

	dupePtr := flag.Bool("duplicates", false, "Check entire database for duplicates")

	flag.Parse()

	localfm.Main(*delayPtr, *limitPtr, *dupePtr)
}
