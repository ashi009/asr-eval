package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

type GroundTruthFile struct {
	GroundTruth string `json:"ground_truth"`
}

func main() {
	id := "3f51a5af-4d2b-41c3-9361-690436422914"
	filename := "transcripts_and_audios/" + id + ".eval.json"

	content, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatal(err)
	}

	var gt GroundTruthFile
	json.Unmarshal(content, &gt)

	gt.GroundTruth += " (MODIFIED GT)"

	newData, _ := json.MarshalIndent(gt, "", "  ")
	ioutil.WriteFile(filename, newData, 0644)
}
