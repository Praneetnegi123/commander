package suite

import (
	"fmt"
	"github.com/SimonBaeumer/commander/pkg/runtime"
	"gopkg.in/yaml.v2"
	"reflect"
	"strings"
)

// YAMLConfig will be used for unmarshalling the yaml test suite
type YAMLConfig struct {
	Tests  map[string]YAMLTest `yaml:"tests"`
	Config YAMLTestConfig      `yaml:"config,omitempty"`
	Nodes  map[string]NodeConf `yaml:"nodes,omitempty"`
}

// YAMLTestConfig is a struct to represent the test config
type YAMLTestConfig struct {
	InheritEnv bool              `yaml:"inherit-env,omitempty"`
	Env        map[string]string `yaml:"env,omitempty"`
	Dir        string            `yaml:"dir,omitempty"`
	Timeout    string            `yaml:"timeout,omitempty"`
	Retries    int               `yaml:"retries,omitempty"`
	Interval   string            `yaml:"interval,omitempty"`
	Nodes      []string          `yaml:"nodes,omitempty"`
}

type NodeConf struct {
	Name         string `yaml:"-"`
	Type         string `yaml:"type"`
	User         string `yaml:"user"`
	Pass         string `yaml:"pass,omitempty"`
	Addr         string `yaml:"addr,omitempty"`
	Image        string `yaml:"image,omitempty"`
	IdentityFile string `yaml:"identity-file,omitempty"`
}

type DockerConf struct {
	Image string `yaml:"image"`
	Name  string `yaml:"name"`
}

// SSHConf represents the target host of the system
type SSHConf struct {
	Host     string `yaml:"host"`
	User     string `yaml:"user"`
	Password string `yaml:"password,omitempty"`
}

// YAMLTest represents a test in the yaml test suite
type YAMLTest struct {
	Title    string         `yaml:"-"`
	Command  string         `yaml:"command,omitempty"`
	ExitCode int            `yaml:"exit-code"`
	Stdout   interface{}    `yaml:"stdout,omitempty"`
	Stderr   interface{}    `yaml:"stderr,omitempty"`
	Config   YAMLTestConfig `yaml:"config,omitempty"`
}

// ParseYAML parses the Suite from a yaml byte slice
func ParseYAML(content []byte) Suite {
	yamlConfig := YAMLConfig{}

	err := yaml.UnmarshalStrict(content, &yamlConfig)
	if err != nil {
		panic(err.Error())
	}

	return Suite{
		TestCases: convertYAMLConfToTestCases(yamlConfig),
		Config: runtime.TestConfig{
			InheritEnv: yamlConfig.Config.InheritEnv,
			Env:        yamlConfig.Config.Env,
			Dir:        yamlConfig.Config.Dir,
			Timeout:    yamlConfig.Config.Timeout,
			Retries:    yamlConfig.Config.Retries,
			Interval:   yamlConfig.Config.Interval,
			Nodes:      yamlConfig.Config.Nodes,
		},
		Nodes: convertNodes(yamlConfig.Nodes),
	}
}

func convertNodes(nodes map[string]NodeConf) []runtime.Node {
	var n []runtime.Node
	for _, v := range nodes {
		n = append(n, runtime.Node{
			Pass:         v.Pass,
			Type:         v.Type,
			User:         v.User,
			Addr:         v.Addr,
			Name:         v.Name,
			Image:        v.Image,
			IdentityFile: v.IdentityFile,
		})
	}
	return n
}

//Convert YAMlConfig to runtime TestCases
func convertYAMLConfToTestCases(conf YAMLConfig) []runtime.TestCase {
	var tests []runtime.TestCase
	for _, t := range conf.Tests {
		tests = append(tests, runtime.TestCase{
			Title: t.Title,
			Command: runtime.CommandUnderTest{
				Cmd:        t.Command,
				InheritEnv: t.Config.InheritEnv,
				Env:        t.Config.Env,
				Dir:        t.Config.Dir,
				Timeout:    t.Config.Timeout,
				Retries:    t.Config.Retries,
				Interval:   t.Config.Interval,
			},
			Expected: runtime.Expected{
				ExitCode: t.ExitCode,
				Stdout:   t.Stdout.(runtime.ExpectedOut),
				Stderr:   t.Stderr.(runtime.ExpectedOut),
			},
			Nodes: t.Config.Nodes,
		})
	}

	return tests
}

// Convert variable to string and remove trailing blank lines
func toString(s interface{}) string {
	return strings.Trim(fmt.Sprintf("%s", s), "\n")
}

// UnmarshalYAML unmarshals the yaml
func (y *YAMLConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var params struct {
		Tests  map[string]YAMLTest `yaml:"tests"`
		Config YAMLTestConfig      `yaml:"config"`
		Nodes  map[string]NodeConf `yaml:"nodes"`
	}

	err := unmarshal(&params)
	if err != nil {
		return err
	}

	// map key to title property
	y.Tests = make(map[string]YAMLTest)
	for k, v := range params.Tests {
		test := YAMLTest{
			Title:    k,
			Command:  v.Command,
			ExitCode: v.ExitCode,
			Stdout:   y.convertToExpectedOut(v.Stdout),
			Stderr:   y.convertToExpectedOut(v.Stderr),
			Config:   y.mergeConfigs(v.Config, params.Config),
		}

		// Set key as command, if command property was empty
		if v.Command == "" {
			test.Command = k
		}

		y.Tests[k] = test
	}

	y.Nodes = make(map[string]NodeConf)
	for k, v := range params.Nodes {
		node := NodeConf{
			Name:         k,
			Addr:         v.Addr,
			User:         v.User,
			Type:         v.Type,
			Pass:         v.Pass,
			IdentityFile: v.IdentityFile,
		}

		y.Nodes[k] = node
	}

	//Parse global configuration
	y.Config = YAMLTestConfig{
		InheritEnv: params.Config.InheritEnv,
		Env:        params.Config.Env,
		Dir:        params.Config.Dir,
		Timeout:    params.Config.Timeout,
		Retries:    params.Config.Retries,
		Interval:   params.Config.Interval,
		Nodes:      params.Config.Nodes,
	}

	return nil
}

//Converts given value to an ExpectedOut. Especially used for Stdout and Stderr.
func (y *YAMLConfig) convertToExpectedOut(value interface{}) runtime.ExpectedOut {
	exp := runtime.ExpectedOut{
		JSON: make(map[string]string),
	}

	switch value.(type) {
	//If only a string was passed it is assigned to exactly automatically
	case string:
		exp.Contains = []string{toString(value)}
		break

	//If there is nested map set the properties will be assigned to the contains
	case map[interface{}]interface{}:
		v := value.(map[interface{}]interface{})
		// Check if keys are parsable
		// TODO: Could be refactored into a registry maybe which holds all parsers
		for k := range v {
			switch k {
			case
				"contains",
				"exactly",
				"line-count",
				"lines",
				"json",
				"xml",
				"not-contains":
				break
			default:
				panic(fmt.Sprintf("Key %s is not allowed.", k))
			}
		}

		//Parse contains key
		if contains := v["contains"]; contains != nil {
			values := contains.([]interface{})
			for _, v := range values {
				exp.Contains = append(exp.Contains, toString(v))
			}
		}

		//Parse exactly key
		if exactly := v["exactly"]; exactly != nil {
			exp.Exactly = toString(exactly)
		}

		//Parse line-count key
		if lc := v["line-count"]; lc != nil {
			exp.LineCount = lc.(int)
		}

		// Parse lines
		if l := v["lines"]; l != nil {
			exp.Lines = make(map[int]string)
			for k, v := range l.(map[interface{}]interface{}) {
				exp.Lines[k.(int)] = toString(v)
			}
		}

		if notContains := v["not-contains"]; notContains != nil {
			values := notContains.([]interface{})
			for _, v := range values {
				exp.NotContains = append(exp.NotContains, toString(v))
			}
		}

		if json := v["json"]; json != nil {
			values := json.(map[interface{}]interface{})
			for k, v := range values {
				exp.JSON[k.(string)] = v.(string)
			}
		}
		break

	case nil:
		break
	default:
		panic(fmt.Sprintf("Failed to parse Stdout or Stderr with values: %v", value))
	}

	return exp
}

// It is needed to create a new map to avoid overwriting the global configuration
func (y *YAMLConfig) mergeConfigs(local YAMLTestConfig, global YAMLTestConfig) YAMLTestConfig {
	conf := global

	conf.Env = y.mergeEnvironmentVariables(global, local)

	if local.Dir != "" {
		conf.Dir = local.Dir
	}

	if local.Timeout != "" {
		conf.Timeout = local.Timeout
	}

	if local.Retries != 0 {
		conf.Retries = local.Retries
	}

	if local.Interval != "" {
		conf.Interval = local.Interval
	}

	if local.InheritEnv {
		conf.InheritEnv = local.InheritEnv
	}

	if len(local.Nodes) != 0 {
		conf.Nodes = local.Nodes
	}

	return conf
}

func (y *YAMLConfig) mergeEnvironmentVariables(global YAMLTestConfig, local YAMLTestConfig) map[string]string {
	env := make(map[string]string)
	for k, v := range global.Env {
		env[k] = v
	}
	for k, v := range local.Env {
		env[k] = v
	}
	return env
}

//MarshalYAML adds custom logic to the struct to yaml conversion
func (y YAMLConfig) MarshalYAML() (interface{}, error) {
	//Detect which values of the stdout/stderr assertions should be filled.
	//If all values are empty except Contains it will convert it to a single string
	//to match the easiest test suite definitions
	for k, t := range y.Tests {
		t.Stdout = convertExpectedOut(t.Stdout.(runtime.ExpectedOut))
		if reflect.ValueOf(t.Stdout).Kind() == reflect.Struct {
			t.Stdout = t.Stdout.(runtime.ExpectedOut)
		}

		t.Stderr = convertExpectedOut(t.Stderr.(runtime.ExpectedOut))
		if reflect.ValueOf(t.Stderr).Kind() == reflect.Struct {
			t.Stderr = t.Stderr.(runtime.ExpectedOut)
		}

		y.Tests[k] = t
	}

	return y, nil
}

func (y *YAMLConfig) mergeNodes(nodes map[string]NodeConf, globalNodes map[string]NodeConf) map[string]NodeConf {
	return nodes
}

func convertExpectedOut(out runtime.ExpectedOut) interface{} {
	//If the property contains consists of only one element it will be set without the struct structure
	if isContainsASingleNonEmptyString(out) && propertiesAreEmpty(out) {
		return out.Contains[0]
	}

	//If the contains property only has one empty string element it should not be displayed
	//in the marshaled yaml file
	if len(out.Contains) == 1 && out.Contains[0] == "" {
		out.Contains = nil
	}

	if len(out.Contains) == 0 && propertiesAreEmpty(out) {
		return nil
	}
	return out
}

func propertiesAreEmpty(out runtime.ExpectedOut) bool {
	return out.Lines == nil &&
		out.Exactly == "" &&
		out.LineCount == 0 &&
		out.NotContains == nil
}

func isContainsASingleNonEmptyString(out runtime.ExpectedOut) bool {
	return len(out.Contains) == 1 &&
		out.Contains[0] != ""
}
