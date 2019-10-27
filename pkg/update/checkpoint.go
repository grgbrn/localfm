package update

import (
	"encoding/json"
	"io/ioutil"

	"bitbucket.org/grgbrn/localfm/pkg/util"
)

const checkpointFilename string = "checkpoint.json"

func writeCheckpoint(path string, state traversalState) error {
	jout, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(path, jout, 0644)
	if err != nil {
		return err
	}
	return nil
}

func checkpointExists() bool {
	return util.FileExists(checkpointFilename)
}

func resumeCheckpoint() (traversalState, error) {
	newState := traversalState{}

	dat, err := ioutil.ReadFile(checkpointFilename)
	if err != nil {
		return newState, err
	}

	if err := json.Unmarshal(dat, &newState); err != nil {
		return newState, err
	}
	return newState, nil
}
