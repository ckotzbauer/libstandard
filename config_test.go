package root

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func TestReadFromEnv(t *testing.T) {
	type Combined struct {
		Empty   int
		Default int `env:"TEST0" env-default:"1"`
		Global  int `env:"TEST1" env-default:"1"`
		local   int `env:"TEST2" env-default:"1"`
	}

	type AllTypes struct {
		Integer         int64             `env:"TEST_INTEGER"`
		UnsInteger      uint64            `env:"TEST_UNSINTEGER"`
		Float           float64           `env:"TEST_FLOAT"`
		Boolean         bool              `env:"TEST_BOOLEAN"`
		String          string            `env:"TEST_STRING"`
		ArrayInt        []int             `env:"TEST_ARRAYINT"`
		ArrayString     []string          `env:"TEST_ARRAYSTRING"`
		MapStringInt    map[string]int    `env:"TEST_MAPSTRINGINT"`
		MapStringString map[string]string `env:"TEST_MAPSTRINGSTRING"`
	}

	type Required struct {
		NotRequired int `env:"NOT_REQUIRED"`
		Required    int `env:"REQUIRED" env-required:"true"`
	}

	tests := []struct {
		name    string
		env     map[string]string
		cfg     interface{}
		want    interface{}
		wantErr bool
	}{
		{
			name: "combined",
			env: map[string]string{
				"TEST1": "2",
				"TEST2": "3",
			},
			cfg: &Combined{},
			want: &Combined{
				Empty:   0,
				Default: 1,
				Global:  2,
				local:   0,
			},
			wantErr: false,
		},

		{
			name: "all types",
			env: map[string]string{
				"TEST_INTEGER":         "-5",
				"TEST_UNSINTEGER":      "5",
				"TEST_FLOAT":           "5.5",
				"TEST_BOOLEAN":         "true",
				"TEST_STRING":          "test",
				"TEST_TIME":            "2012-04-23T18:25:43.511Z",
				"TEST_ARRAYINT":        "1,2,3",
				"TEST_ARRAYSTRING":     "a,b,c",
				"TEST_MAPSTRINGINT":    "a:1,b:2,c:3",
				"TEST_MAPSTRINGSTRING": "a:x,b:y,c:z",
			},
			cfg: &AllTypes{},
			want: &AllTypes{
				Integer:     -5,
				UnsInteger:  5,
				Float:       5.5,
				Boolean:     true,
				String:      "test",
				ArrayInt:    []int{1, 2, 3},
				ArrayString: []string{"a", "b", "c"},
				MapStringInt: map[string]int{
					"a": 1,
					"b": 2,
					"c": 3,
				},
				MapStringString: map[string]string{
					"a": "x",
					"b": "y",
					"c": "z",
				},
			},
			wantErr: false,
		},

		{
			name: "wrong types",
			env: map[string]string{
				"TEST_INTEGER":         "a",
				"TEST_UNSINTEGER":      "b",
				"TEST_FLOAT":           "c",
				"TEST_BOOLEAN":         "xxx",
				"TEST_STRING":          "",
				"TEST_ARRAYINT":        "a,b,c",
				"TEST_ARRAYSTRING":     "1,2,3",
				"TEST_MAPSTRINGINT":    "a:x,b:y,c:z",
				"TEST_MAPSTRINGSTRING": "a:1,b:2,c:3",
			},
			cfg:     &AllTypes{},
			want:    &AllTypes{},
			wantErr: true,
		},

		{
			name: "wrong int",
			env: map[string]string{
				"TEST_INTEGER": "a",
			},
			cfg:     &AllTypes{},
			want:    &AllTypes{},
			wantErr: true,
		},

		{
			name: "wrong uint",
			env: map[string]string{
				"TEST_UNSINTEGER": "b",
			},
			cfg:     &AllTypes{},
			want:    &AllTypes{},
			wantErr: true,
		},

		{
			name: "wrong float",
			env: map[string]string{
				"TEST_FLOAT": "c",
			},
			cfg:     &AllTypes{},
			want:    &AllTypes{},
			wantErr: true,
		},

		{
			name: "wrong boolean",
			env: map[string]string{
				"TEST_BOOLEAN": "xxx",
			},
			cfg:     &AllTypes{},
			want:    &AllTypes{},
			wantErr: true,
		},

		{
			name: "wrong array int",
			env: map[string]string{
				"TEST_ARRAYINT": "a,b,c",
			},
			cfg:     &AllTypes{},
			want:    &AllTypes{},
			wantErr: true,
		},

		{
			name: "wrong map int",
			env: map[string]string{
				"TEST_MAPSTRINGINT": "a:x,b:y,c:z",
			},
			cfg:     &AllTypes{},
			want:    &AllTypes{},
			wantErr: true,
		},

		{
			name: "wrong map type int",
			env: map[string]string{
				"TEST_MAPSTRINGINT": "-",
			},
			cfg:     &AllTypes{},
			want:    &AllTypes{},
			wantErr: true,
		},

		{
			name: "wrong map type string",
			env: map[string]string{
				"TEST_MAPSTRINGSTRING": "-",
			},
			cfg:     &AllTypes{},
			want:    &AllTypes{},
			wantErr: true,
		},

		{
			name:    "wrong config type",
			cfg:     42,
			want:    42,
			wantErr: true,
		},

		{
			name:    "required error",
			cfg:     &Required{},
			want:    &Required{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for env, val := range tt.env {
				os.Setenv(env, val)
			}
			defer os.Clearenv()

			if err := ReadFromEnv(tt.cfg); (err != nil) != tt.wantErr {
				t.Errorf("wrong error behavior %v, wantErr %v", err, tt.wantErr)
			}

			if !reflect.DeepEqual(tt.cfg, tt.want) {
				t.Errorf("wrong data %v, want %v", tt.cfg, tt.want)
			}
		})
	}
}

func TestReadFromEnvWithPrefix(t *testing.T) {
	type Logging struct {
		Debug bool `env:"DEBUG"`
	}

	type DBConfig struct {
		Host    string  `env:"DB_HOST"`
		Port    int     `env:"DB_PORT"`
		Logging Logging `env-prefix:"DB_"`
	}

	type Config struct {
		Default  DBConfig
		ReadOnly DBConfig `env-prefix:"READONLY_"`
		Extra    DBConfig `env-prefix:"EXTRA_"`
	}

	var env = map[string]string{
		"DB_HOST":           "db1.host",
		"DB_PORT":           "10000",
		"DB_DEBUG":          "true",
		"READONLY_DB_HOST":  "db2.host",
		"READONLY_DB_PORT":  "20000",
		"READONLY_DB_DEBUG": "true",
		"EXTRA_DB_HOST":     "db3.host",
		"EXTRA_DB_PORT":     "30000",
		"EXTRA_DB_DEBUG":    "true",
	}
	for k, v := range env {
		os.Setenv(k, v)
	}

	var cfg Config
	if err := ReadFromEnv(&cfg); err != nil {
		t.Fatal("failed to read env vars", err)
	}

	var expected = Config{
		Default: DBConfig{
			Host:    "db1.host",
			Port:    10000,
			Logging: Logging{Debug: true},
		},
		ReadOnly: DBConfig{
			Host:    "db2.host",
			Port:    20000,
			Logging: Logging{Debug: true},
		},
		Extra: DBConfig{
			Host:    "db3.host",
			Port:    30000,
			Logging: Logging{Debug: true},
		},
	}

	if !reflect.DeepEqual(cfg, expected) {
		t.Errorf("wrong data %v, want %v", cfg, expected)
	}
}

func TestReadFromFlags(t *testing.T) {
	type Config struct {
		Host string `flag:"host"`
		Port int32  `flag:"port"`
	}

	flagSet := &pflag.FlagSet{}
	flagSet.String("host", "google.de", "Host-Flag")
	flagSet.Int32("port", 5432, "Port-Flag")
	err := flagSet.Set("port", "1000")
	assert.Nil(t, err)

	var cfg Config
	if err := ReadFromFlags(&cfg, flagSet); err != nil {
		t.Fatal("failed to read flag vars", err)
	}

	var expected = Config{
		Host: "google.de",
		Port: 1000,
	}

	if !reflect.DeepEqual(cfg, expected) {
		t.Errorf("wrong data %v, want %v", cfg, expected)
	}
}

type flagSetting struct {
	defaultValue string
	value        string
}

func TestReadFromFlagsWithEnvs(t *testing.T) {
	type config struct {
		Number    string `flag:"number" env:"TEST_NUMBER" env-default:"1"`
		String    string `flag:"string" env:"TEST_STRING" env-default:"default"`
		NoDefault string `flag:"no-default" env:"TEST_NO_DEFAULT"`
		NoEnv     string `flag:"no-env" env-default:"default"`
	}

	tests := []struct {
		name    string
		flags   map[string]flagSetting
		env     map[string]string
		want    *config
		wantErr bool
	}{
		{
			name: "flags_only",
			flags: map[string]flagSetting{
				"number": {value: "2", defaultValue: "3"},
				"string": {value: "test", defaultValue: ""},
				"no-env": {value: "", defaultValue: "those"},
			},
			env: nil,
			want: &config{
				Number:    "2",
				String:    "test",
				NoDefault: "",
				NoEnv:     "those",
			},
			wantErr: false,
		},

		{
			name:  "env_only",
			flags: nil,
			env: map[string]string{
				"TEST_NUMBER": "2",
				"TEST_STRING": "test",
			},
			want: &config{
				Number:    "2",
				String:    "test",
				NoDefault: "",
				NoEnv:     "default",
			},
			wantErr: false,
		},

		{
			name: "flags_and_env",
			flags: map[string]flagSetting{
				"number":     {value: "2", defaultValue: ""},
				"no-default": {value: "", defaultValue: "flagdefault"},
				"no-env":     {value: "", defaultValue: "DefaultFromFlag"},
			},
			env: map[string]string{
				"TEST_NUMBER":     "3",
				"TEST_STRING":     "fromEnv",
				"TEST_NO_DEFAULT": "value",
			},
			want: &config{
				Number:    "2",
				String:    "fromEnv",
				NoDefault: "value",
				NoEnv:     "DefaultFromFlag",
			},
			wantErr: false,
		},

		{
			name:  "empty",
			flags: nil,
			env:   nil,
			want: &config{
				Number:    "1",
				String:    "default",
				NoDefault: "",
				NoEnv:     "default",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for env, val := range tt.env {
				os.Setenv(env, val)
			}
			defer os.Clearenv()

			flagSet := &pflag.FlagSet{}
			for flag, val := range tt.flags {
				flagSet.String(flag, val.defaultValue, "")
				if val.value != "" {
					err := flagSet.Set(flag, val.value)
					assert.Nil(t, err)
				}
			}

			var cfg config
			var err error
			if err := ReadFromFlags(&cfg, flagSet); (err != nil) != tt.wantErr {
				t.Errorf("wrong error behavior %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && !reflect.DeepEqual(&cfg, tt.want) {
				t.Errorf("wrong data %v, want %v", &cfg, tt.want)
			}
		})
	}
}

func TestReadFromFile(t *testing.T) {
	type configObject struct {
		One int `yaml:"one" json:"one"`
		Two int `yaml:"two" json:"two"`
	}
	type config struct {
		Number  int64        `yaml:"number" json:"number"`
		Float   float64      `yaml:"float" json:"float"`
		String  string       `yaml:"string" json:"string"`
		Boolean bool         `yaml:"boolean" json:"boolean"`
		Object  configObject `yaml:"object" json:"object"`
		Array   []int        `yaml:"array" json:"array"`
	}

	wantConfig := config{
		Number:  1,
		Float:   2.3,
		String:  "test",
		Boolean: true,
		Object:  configObject{1, 2},
		Array:   []int{1, 2, 3},
	}

	tests := []struct {
		name    string
		file    string
		ext     string
		want    *config
		wantErr bool
	}{
		{
			name: "yaml",
			file: `
number: 1
float: 2.3
string: test
boolean: yes
object:
  one: 1
  two: 2
array: [1, 2, 3]`,
			ext:     "yaml",
			want:    &wantConfig,
			wantErr: false,
		},

		{
			name: "json",
			file: `{
	"number": 1,
	"float": 2.3,
	"string": "test",
	"boolean": true,
	"object": {
		"one": 1,
		"two": 2
	},
	"array": [1, 2, 3]
}`,
			ext:     "json",
			want:    &wantConfig,
			wantErr: false,
		},

		{
			name:    "unknown",
			file:    "-",
			ext:     "",
			want:    nil,
			wantErr: true,
		},

		{
			name:    "parsing error",
			file:    "-",
			ext:     "json",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := ioutil.TempFile(os.TempDir(), fmt.Sprintf("*.%s", tt.ext))
			if err != nil {
				t.Fatal("cannot create temporary file:", err)
			}
			defer os.Remove(tmpFile.Name())

			text := []byte(tt.file)
			if _, err = tmpFile.Write(text); err != nil {
				t.Fatal("failed to write to temporary file:", err)
			}

			var cfg config
			if err = ReadFromFile(&cfg, tmpFile.Name(), DefaultFileConfig{}); (err != nil) != tt.wantErr {
				t.Errorf("wrong error behavior %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && !reflect.DeepEqual(&cfg, tt.want) {
				t.Errorf("wrong data %v, want %v", &cfg, tt.want)
			}
		})
	}

	t.Run("invalid path", func(t *testing.T) {
		err := parseFile("invalid file path", nil)
		if err == nil {
			t.Error("expected error for invalid file path")
		}
	})
}

func TestReadFromDefaultFile(t *testing.T) {
	type configObject struct {
		One int `yaml:"one" json:"one"`
		Two int `yaml:"two" json:"two"`
	}

	wantConfig := configObject{
		One: 1,
		Two: 2,
	}

	tests := []struct {
		name       string
		defaultCfg DefaultFileConfig
		jsonFile   string
		yamlFile   string
		want       *configObject
		wantErr    bool
	}{
		{
			name:       "yaml",
			defaultCfg: DefaultFileConfig{Name: "mycfg", Extensions: []string{"yaml"}, Paths: []string{"/tmp"}},
			yamlFile: `
one: 1
two: 2`,
			want:    &wantConfig,
			wantErr: false,
		},

		{
			name:       "json",
			defaultCfg: DefaultFileConfig{Name: "mycfg", Extensions: []string{"json"}, Paths: []string{"/tmp"}},
			jsonFile: `{
	"one": 1,
	"two": 2
}`,
			want:    &wantConfig,
			wantErr: false,
		},

		{
			name:       "json_or_yaml",
			defaultCfg: DefaultFileConfig{Name: "mycfg", Extensions: []string{"json", "yaml"}, Paths: []string{"/tmp"}},
			yamlFile: `
one: 1
two: 2`,
			want:    &wantConfig,
			wantErr: false,
		},

		{
			name:       "no file",
			defaultCfg: DefaultFileConfig{},
			want:       &configObject{},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, ext := range tt.defaultCfg.Extensions {
				var text []byte

				if ext == "json" {
					text = []byte(tt.jsonFile)
				} else {
					text = []byte(tt.yamlFile)
				}

				if len(text) > 0 {
					tmpFile, err := os.Create(filepath.Join(tt.defaultCfg.Paths[0], tt.defaultCfg.Name+"."+ext))
					if err != nil {
						t.Fatal("cannot create temporary file:", err)
					}
					defer os.Remove(tmpFile.Name())

					if _, err = tmpFile.Write(text); err != nil {
						t.Fatal("failed to write to temporary file:", err)
					}
				}
			}

			var cfg configObject
			var err error
			if err = ReadFromFile(&cfg, "", tt.defaultCfg); (err != nil) != tt.wantErr {
				t.Errorf("wrong error behavior %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && !reflect.DeepEqual(&cfg, tt.want) {
				t.Errorf("wrong data %v, want %v", &cfg, tt.want)
			}
		})
	}
}

func TestReadFromFileWithEnvs(t *testing.T) {
	type config struct {
		Number    int64  `yaml:"number" env:"TEST_NUMBER" env-default:"1"`
		String    string `yaml:"string" env:"TEST_STRING" env-default:"default"`
		NoDefault string `yaml:"no-default" env:"TEST_NO_DEFAULT"`
		NoEnv     string `yaml:"no-env" env-default:"default"`
	}

	tests := []struct {
		name    string
		file    string
		ext     string
		env     map[string]string
		want    *config
		wantErr bool
	}{
		{
			name: "yaml_only",
			file: `
number: 2
string: test
no-default: NoDefault
no-env: this
`,
			ext: "yaml",
			env: nil,
			want: &config{
				Number:    2,
				String:    "test",
				NoDefault: "NoDefault",
				NoEnv:     "this",
			},
			wantErr: false,
		},

		{
			name: "env_only",
			file: "none: none",
			ext:  "yaml",
			env: map[string]string{
				"TEST_NUMBER": "2",
				"TEST_STRING": "test",
			},
			want: &config{
				Number:    2,
				String:    "test",
				NoDefault: "",
				NoEnv:     "default",
			},
			wantErr: false,
		},

		{
			name: "yaml_and_env",
			file: `
number: 2
string: test
no-default: NoDefault
no-env: this
`,
			ext: "yaml",
			env: map[string]string{
				"TEST_NUMBER": "3",
				"TEST_STRING": "fromEnv",
			},
			want: &config{
				Number:    3,
				String:    "fromEnv",
				NoDefault: "NoDefault",
				NoEnv:     "this",
			},
			wantErr: false,
		},

		{
			name: "empty",
			file: "none: none",
			ext:  "yaml",
			env:  nil,
			want: &config{
				Number:    1,
				String:    "default",
				NoDefault: "",
				NoEnv:     "default",
			},
			wantErr: false,
		},

		{
			name:    "unknown",
			file:    "-",
			ext:     "",
			want:    nil,
			wantErr: true,
		},

		{
			name:    "parsing error",
			file:    "-",
			ext:     "json",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := ioutil.TempFile(os.TempDir(), fmt.Sprintf("*.%s", tt.ext))
			if err != nil {
				t.Fatal("cannot create temporary file:", err)
			}
			defer os.Remove(tmpFile.Name())

			text := []byte(tt.file)
			if _, err = tmpFile.Write(text); err != nil {
				t.Fatal("failed to write to temporary file:", err)
			}

			for env, val := range tt.env {
				os.Setenv(env, val)
			}
			defer os.Clearenv()

			var cfg config
			if err = ReadFromFile(&cfg, tmpFile.Name(), DefaultFileConfig{}); (err != nil) != tt.wantErr {
				t.Errorf("wrong error behavior %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && !reflect.DeepEqual(&cfg, tt.want) {
				t.Errorf("wrong data %v, want %v", &cfg, tt.want)
			}
		})
	}
}

func TestAddConfigFlag(t *testing.T) {
	cmd := &cobra.Command{}
	AddConfigFlag(cmd)
	assert.NotNil(t, cmd.PersistentFlags().Lookup(Config))
}
