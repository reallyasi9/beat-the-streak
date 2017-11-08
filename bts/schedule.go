package bts

import (
	"fmt"
	"io/ioutil"

	yaml "gopkg.in/yaml.v2"
)

type Schedule map[Team][]Team

func MakeSchedule(fileName string) (Schedule, error) {

	schedYaml, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	s := make(Schedule)
	err = yaml.Unmarshal(schedYaml, s)
	if err != nil {
		return nil, err
	}

	for k, v := range s {
		if len(v) != 13 {
			return nil, fmt.Errorf("schedule for team %s incorrect: expected %d, got %d", k, 13, len(v))
		}
	}

	return s, nil
}
