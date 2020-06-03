// Copyright 2020 Jaume Martin

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// 	http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package rules

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mole-ids/mole/internal/merr"
	"github.com/mole-ids/mole/pkg/logger"
	"github.com/pkg/errors"
)

// Manager stores the rules and manages everything related with rules
type Manager struct {
	// Config manger's configuration most of its values come from the arguments
	// or configuration file
	Config *Config
	// RawRules store all Yara rules
	RawRules []string
}

// NewManager returns a new rules manager
func NewManager() (manager *Manager, err error) {
	manager = &Manager{}
	manager.Config, err = InitConfig()

	if err != nil {
		return nil, errors.Wrap(err, merr.InitRulesManagerMsg)
	}

	// Load rules
	err = manager.LoadRules()
	if err != nil {
		return nil, errors.Wrap(err, merr.LoadingRulesMsg)
	}

	logger.Log.Info(logger.YaraRulesInitiatedMsg)

	return manager, err
}

const (
	yaraFileGlob = "*.yar"
)

var (
	varRe          = regexp.MustCompile(`(?i)\$\w+`)
	includeRe      = regexp.MustCompile(`(?i)\s*include\s+`)
	removeBlanksRe = regexp.MustCompile(`[\t\r\n]+`)
	// splitRE        = regexp.MustCompile(`(?img)rule(?:[^\n}]|\n[^\n])+`)
	splitRE = regexp.MustCompile(`(?im)}\s*rule`)

	// the following regexp are used to pre-procces the rules
	srcAnyPreprocRE     = regexp.MustCompile(`src\s*=\s*"any"`)
	srcPortAnyPreprocRE = regexp.MustCompile(`src_port\s*=\s*"any"`)
	dstAnyPreprocRE     = regexp.MustCompile(`dst\s*=\s*"any"`)
	dstPortAnyPreprocRE = regexp.MustCompile(`dst_port\s*=\s*"any"`)
)

// LoadRules load the rules defined either in the rulesIndex or rulesDir flags
func (ma *Manager) LoadRules() (err error) {
	if ma.Config.RulesIndex == "" && ma.Config.RulesFolder == "" {
		return merr.ErrRuleOrIndexNotDefined
	}

	if ma.Config.RulesIndex != "" {
		logger.Log.Infof(logger.RulesIndexFileMsg, ma.Config.RulesIndex)
		ma.loadRulesByIndex()
	}

	if ma.Config.RulesFolder != "" {
		logger.Log.Infof(logger.RulesFolderMsg, ma.Config.RulesFolder)
		ma.loadRulesByDir()
	}

	logger.Log.Infof(logger.YaraRulesLoadedMsg, len(ma.RawRules))

	return nil
}

// loadRulesByIndex loads the rules defined in the `idxFile`
func (ma *Manager) loadRulesByIndex() (err error) {
	idxFile := ma.Config.RulesIndex
	// Removing comments from the file
	res, err := removeCAndCppCommentsFile(idxFile)
	if err != nil {
		return errors.Wrap(err, merr.WhileLoadingRulesMsg)
	}
	cleanIndex := string(res)
	// Removing empty lines
	cleanIndex = removeBlanksRe.ReplaceAllString(strings.TrimSpace(cleanIndex), "\n")

	lines := strings.Split(cleanIndex, "\n")

	fmt.Printf("index file: %s\ncontent: %s\n", idxFile, cleanIndex)

	// Get the base path of the index file
	base := filepath.Dir(idxFile)

	if err != nil {
		return errors.Wrapf(err, merr.AbsIndexPathMsg, idxFile)
	}

	for _, iline := range lines {
		line := cleanUpLine(iline)

		// Get the final rule path
		rulePath := filepath.Join(base, line)

		// Read the rule content based on the rule file real file
		ruleString, err := ioutil.ReadFile(rulePath)
		if err != nil {
			return errors.Wrapf(err, merr.YaraReadFileMsg, rulePath)
		}

		ma.readRuleByRule(ruleString)
	}

	return nil
}

// loadRulesByDir loads the rules (files *.yar) placed in `rulesFolder`
func (ma *Manager) loadRulesByDir() (err error) {
	rulesFolder := ma.Config.RulesFolder
	files, err := loadFiles(rulesFolder)
	if err != nil {
		return errors.Wrap(err, merr.OpenRulesDirMsg)
	}

	for _, file := range files {
		ruleString, err := ioutil.ReadFile(file)
		if err != nil {
			return errors.Wrapf(err, merr.OpenRuleFilesMsg, file)
		}

		ma.readRuleByRule(ruleString)
	}

	return nil
}

func (ma *Manager) readRuleByRule(rule []byte) {
	rules := splitRules(string(rule))

	for _, rule := range rules {
		newRule := parseRuleAndVars(rule, ma.Config.Vars)
		ma.RawRules = append(ma.RawRules, newRule)
	}
}

// GetRawRules retuns the loaded rules in raw format
func (ma *Manager) GetRawRules() []string {
	return ma.RawRules
}
