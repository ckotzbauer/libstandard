package libstandard

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type DefaultFileConfig struct {
	Name       string
	Extensions []string
	Paths      []string
}

func AddConfigFlag(cmd *cobra.Command) {
	cmd.PersistentFlags().StringP(Config, "c", "", "Path to the config-file.")
}

// ReadFromEnv reads configuration from environment variables, parses them depending on tags in structure provided.
// Then it reads and parses
//
// Example:
//
//	 type ConfigDatabase struct {
//	 	Port     string `env:"PORT" env-default:"5432"`
//	 	Host     string `env:"HOST" env-default:"localhost"`
//	 	Name     string `env:"NAME" env-default:"postgres"`
//	 	User     string `env:"USER" env-default:"user"`
//	 	Password string `env:"PASSWORD"`
//	 }
//
//	 var cfg ConfigDatabase
//
//	 err := config.ReadFromEnv(&cfg)
//	 if err != nil {
//	     ...
//	 }
func ReadFromEnv(cfg interface{}) error {
	return Read(cfg, nil, "", DefaultFileConfig{})
}

// ReadFromFile reads configuration from a file and environment variables, parses them depending on tags in structure provided.
// Then it reads and parses
//
// Example:
//
//	 type ConfigDatabase struct {
//	 	Port     string `yaml:"port" env:"PORT" env-default:"5432"`
//	 	Host     string `yaml:"host" env:"HOST" env-default:"localhost"`
//	 	Name     string `yaml:"name" env:"NAME" env-default:"postgres"`
//	 	User     string `yaml:"user" env:"USER" env-default:"user"`
//	 	Password string `yaml:"password" env:"PASSWORD"`
//	 }
//
//	 var cfg ConfigDatabase
//
//	 err := config.ReadFromFile(&cfg, "config.yml", DefaultFileConfig{})
//	 if err != nil {
//	     ...
//	 }
func ReadFromFile(cfg interface{}, file string, defaultCfg DefaultFileConfig) error {
	return Read(cfg, nil, file, defaultCfg)
}

// ReadFromFlags reads configuration from environment variables and cmd-flags, parses them depending on tags in structure provided.
// Then it reads and parses
//
// Example:
//
//	 type ConfigDatabase struct {
//	 	Port     string `flag:"port" env:"PORT" env-default:"5432"`
//	 	Host     string `flag:"host" env:"HOST" env-default:"localhost"`
//	 	Name     string `flag:"name" env:"NAME" env-default:"postgres"`
//	 	User     string `flag:"user" env:"USER" env-default:"user"`
//	 	Password string `flag:"password" env:"PASSWORD"`
//	 }
//
//	 var cfg ConfigDatabase
//
//	 cmd.Flags().Int32("port", 5432, "Server-Port")
//	 ...
//
//	 err := config.ReadFromFlags(&cfg, cmd.Flags())
//	 if err != nil {
//	     ...
//	 }
func ReadFromFlags(cfg interface{}, flags *pflag.FlagSet) error {
	return Read(cfg, flags, "", DefaultFileConfig{})
}

// Read reads configuration from a file, environment variables and cmd-flags, parses them depending on tags in structure provided.
// Then it reads and parses
//
// Example:
//
//	 type ConfigDatabase struct {
//	 	Port     string `flag:"port" yaml:"port" env:"PORT" env-default:"5432"`
//	 	Host     string `flag:"host" yaml:"host" env:"HOST" env-default:"localhost"`
//	 	Name     string `flag:"name" yaml:"name" env:"NAME" env-default:"postgres"`
//	 	User     string `flag:"user" yaml:"user" env:"USER" env-default:"user"`
//	 	Password string `flag:"password" yaml:"password" env:"PASSWORD"`
//	 }
//
//	 var cfg ConfigDatabase
//
//	 cmd.Flags().Int32("port", 5432, "Server-Port")
//	 ...
//
//	 err := config.Read(&cfg, cmd.Flags(), "config.yml", , DefaultFileConfig{})
//	 if err != nil {
//	     ...
//	 }
func Read(cfg interface{}, flags *pflag.FlagSet, file string, defaultCfg DefaultFileConfig) error {
	metaInfo, err := readStructMetadata(cfg)
	if err != nil {
		return err
	}

	if file == "" {
		file = findDefaultFile(defaultCfg)
	}

	if file != "" {
		err = parseFile(file, cfg)
		if err != nil {
			return err
		}
	}

	err = readEnvVars(cfg, metaInfo)
	if err != nil {
		return err
	}

	if flags != nil {
		err = parseFlags(flags, cfg, metaInfo)
		if err != nil {
			return err
		}
	}

	return checkRequired(metaInfo)
}

const (
	// DefaultSeparator is a default list and map separator character
	DefaultSeparator = ","
)

// Supported tags
const (
	// Name of the environment variable or a list of names
	TagEnv = "env"
	// Default value
	TagEnvDefault = "env-default"
	// Flag name
	TagFlagName = "flag"
	// Custom list and map separator
	TagEnvSeparator = "env-separator"
	// Flag to mark a field as required
	TagEnvRequired = "env-required"
	// Flag to specify prefix for structure fields
	TagEnvPrefix = "env-prefix"
)

// Setter is an interface for a custom value setter.
//
// To implement a custom value setter you need to add a SetValue function to your type that will receive a string raw value:
//
// 	type MyField string
//
// 	func (f *MyField) SetValue(s string) error {
// 		if s == "" {
// 			return fmt.Errorf("field value can't be empty")
// 		}
// 		*f = MyField("my field is: " + s)
// 		return nil
// 	}
type Setter interface {
	SetValue(string) error
}

func findDefaultFile(defaultCfg DefaultFileConfig) string {
	for _, p := range defaultCfg.Paths {
		for _, ext := range defaultCfg.Extensions {
			fullPath := filepath.Join(p, defaultCfg.Name+"."+ext)
			home, _ := os.UserHomeDir()
			if strings.HasPrefix(fullPath, "~/") {
				fullPath = filepath.Join(home, fullPath[2:])
			}

			if _, err := os.Stat(fullPath); err == nil {
				return fullPath
			}
		}
	}
	return ""
}

func checkRequired(metaInfo []structMeta) error {
	for _, meta := range metaInfo {
		if meta.required && meta.isFieldValueZero() {
			err := fmt.Errorf("field %q is required but the value is not provided",
				meta.fieldName)
			return err
		}
	}

	return nil
}

func parseFlags(flags *pflag.FlagSet, cfg interface{}, metaInfo []structMeta) error {
	for _, meta := range metaInfo {
		var rawValue *string

		if meta.flagName != "" {
			flag := flags.Lookup(meta.flagName)
			if flag != nil && flag.Changed {
				s := flag.Value.String()
				rawValue = &s
			} else if flag != nil && flag.DefValue != "" && (meta.isFieldValueZero() || (meta.defValue != nil && meta.fieldValue.String() == *meta.defValue)) {
				rawValue = &flag.DefValue
			}
		}

		if rawValue == nil && meta.isFieldValueZero() {
			rawValue = meta.defValue
		}

		if rawValue == nil {
			continue
		}

		if err := parseValue(meta.fieldValue, *rawValue, meta.separator); err != nil {
			return err
		}
	}

	return nil
}

// parseFile parses configuration file according to it's extension
//
// Currently following file extensions are supported:
//
// - yaml
//
// - json
func parseFile(path string, cfg interface{}) error {
	// open the configuration file
	/* #nosec */
	f, err := os.OpenFile(path, os.O_RDONLY|os.O_SYNC, 0)
	if err != nil {
		return err
	}

	/* #nosec */
	defer f.Close()

	// parse the file depending on the file type
	switch ext := strings.ToLower(filepath.Ext(path)); ext {
	case ".yaml", ".yml":
		err = parseYAML(f, cfg)
	case ".json":
		err = parseJSON(f, cfg)
	default:
		return fmt.Errorf("file format '%s' doesn't supported by the parser", ext)
	}
	if err != nil {
		return fmt.Errorf("config file parsing error: %s", err.Error())
	}
	return nil
}

// parseYAML parses YAML from reader to data structure
func parseYAML(r io.Reader, str interface{}) error {
	return yaml.NewDecoder(r).Decode(str)
}

// parseJSON parses JSON from reader to data structure
func parseJSON(r io.Reader, str interface{}) error {
	return json.NewDecoder(r).Decode(str)
}

// structMeta is a structure metadata entity
type structMeta struct {
	envList    []string
	flagName   string
	fieldName  string
	fieldValue reflect.Value
	defValue   *string
	separator  string
	required   bool
}

// isFieldValueZero determines if fieldValue empty or not
func (sm *structMeta) isFieldValueZero() bool {
	return isZero(sm.fieldValue)
}

// readStructMetadata reads structure metadata (types, tags, etc.)
func readStructMetadata(cfgRoot interface{}) ([]structMeta, error) {
	type cfgNode struct {
		Val    interface{}
		Prefix string
	}

	cfgStack := []cfgNode{{cfgRoot, ""}}
	metas := make([]structMeta, 0)

	for i := 0; i < len(cfgStack); i++ {

		s := reflect.ValueOf(cfgStack[i].Val)
		sPrefix := cfgStack[i].Prefix

		// unwrap pointer
		if s.Kind() == reflect.Ptr {
			s = s.Elem()
		}

		// process only structures
		if s.Kind() != reflect.Struct {
			return nil, fmt.Errorf("wrong type %v", s.Kind())
		}
		typeInfo := s.Type()

		// read tags
		for idx := 0; idx < s.NumField(); idx++ {
			fType := typeInfo.Field(idx)

			var (
				defValue  *string
				flagName  string
				separator string
			)

			// process nested structure
			if fld := s.Field(idx); fld.Kind() == reflect.Struct {
				prefix, _ := fType.Tag.Lookup(TagEnvPrefix)
				cfgStack = append(cfgStack, cfgNode{fld.Addr().Interface(), sPrefix + prefix})
			}

			// check is the field value can be changed
			if !s.Field(idx).CanSet() {
				continue
			}

			if def, ok := fType.Tag.Lookup(TagEnvDefault); ok {
				defValue = &def
			}

			if flag, ok := fType.Tag.Lookup(TagFlagName); ok {
				flagName = flag
			}

			if sep, ok := fType.Tag.Lookup(TagEnvSeparator); ok {
				separator = sep
			} else {
				separator = DefaultSeparator
			}

			_, required := fType.Tag.Lookup(TagEnvRequired)

			envList := make([]string, 0)

			if envs, ok := fType.Tag.Lookup(TagEnv); ok && len(envs) != 0 {
				envList = strings.Split(envs, DefaultSeparator)
				if sPrefix != "" {
					for i := range envList {
						envList[i] = sPrefix + envList[i]
					}
				}
			}

			metas = append(metas, structMeta{
				envList:    envList,
				flagName:   flagName,
				fieldName:  s.Type().Field(idx).Name,
				fieldValue: s.Field(idx),
				defValue:   defValue,
				separator:  separator,
				required:   required,
			})
		}

	}

	return metas, nil
}

// readEnvVars reads environment variables to the provided configuration structure
func readEnvVars(cfg interface{}, metaInfo []structMeta) error {
	for _, meta := range metaInfo {
		var rawValue *string

		for _, env := range meta.envList {
			if value, ok := os.LookupEnv(env); ok {
				rawValue = &value
				break
			}
		}

		if rawValue == nil && meta.isFieldValueZero() {
			rawValue = meta.defValue
		}

		if rawValue == nil {
			continue
		}

		if err := parseValue(meta.fieldValue, *rawValue, meta.separator); err != nil {
			return err
		}
	}

	return nil
}

// parseValue parses value into the corresponding field.
// In case of maps and slices it uses provided separator to split raw value string
func parseValue(field reflect.Value, value, sep string) error {
	// TODO: simplify recursion

	if field.CanInterface() {
		if cs, ok := field.Interface().(Setter); ok {
			return cs.SetValue(value)
		} else if csp, ok := field.Addr().Interface().(Setter); ok {
			return csp.SetValue(value)
		}
	}

	valueType := field.Type()

	switch valueType.Kind() {
	// parse string value
	case reflect.String:
		field.SetString(value)

	// parse boolean value
	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		field.SetBool(b)

	// parse integer (or time) value
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// parse regular integer
		number, err := strconv.ParseInt(value, 0, valueType.Bits())
		if err != nil {
			return err
		}
		field.SetInt(number)

	// parse unsigned integer value
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		number, err := strconv.ParseUint(value, 0, valueType.Bits())
		if err != nil {
			return err
		}
		field.SetUint(number)

	// parse floating point value
	case reflect.Float32, reflect.Float64:
		number, err := strconv.ParseFloat(value, valueType.Bits())
		if err != nil {
			return err
		}
		field.SetFloat(number)

	// parse sliced value
	case reflect.Slice:
		sliceValue, err := parseSlice(valueType, value, sep)
		if err != nil {
			return err
		}

		field.Set(*sliceValue)

	// parse mapped value
	case reflect.Map:
		mapValue, err := parseMap(valueType, value, sep)
		if err != nil {
			return err
		}

		field.Set(*mapValue)

	default:
		return fmt.Errorf("unsupported type %s.%s", valueType.PkgPath(), valueType.Name())
	}

	return nil
}

// parseSlice parses value into a slice of given type
func parseSlice(valueType reflect.Type, value string, sep string) (*reflect.Value, error) {
	sliceValue := reflect.MakeSlice(valueType, 0, 0)
	if valueType.Elem().Kind() == reflect.Uint8 {
		sliceValue = reflect.ValueOf([]byte(value))
	} else if len(strings.TrimSpace(value)) != 0 {
		values := strings.Split(value, sep)
		sliceValue = reflect.MakeSlice(valueType, len(values), len(values))

		for i, val := range values {
			if err := parseValue(sliceValue.Index(i), val, sep); err != nil {
				return nil, err
			}
		}
	}
	return &sliceValue, nil
}

// parseMap parses value into a map of given type
func parseMap(valueType reflect.Type, value string, sep string) (*reflect.Value, error) {
	mapValue := reflect.MakeMap(valueType)
	if len(strings.TrimSpace(value)) != 0 {
		pairs := strings.Split(value, sep)
		for _, pair := range pairs {
			kvPair := strings.SplitN(pair, ":", 2)
			if len(kvPair) != 2 {
				return nil, fmt.Errorf("invalid map item: %q", pair)
			}
			k := reflect.New(valueType.Key()).Elem()
			err := parseValue(k, kvPair[0], sep)
			if err != nil {
				return nil, err
			}
			v := reflect.New(valueType.Elem()).Elem()
			err = parseValue(v, kvPair[1], sep)
			if err != nil {
				return nil, err
			}
			mapValue.SetMapIndex(k, v)
		}
	}
	return &mapValue, nil
}

// isZero is a backport of reflect.Value.IsZero()
func isZero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return math.Float64bits(v.Float()) == 0
	case reflect.Complex64, reflect.Complex128:
		c := v.Complex()
		return math.Float64bits(real(c)) == 0 && math.Float64bits(imag(c)) == 0
	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if !isZero(v.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice, reflect.UnsafePointer:
		return v.IsNil()
	case reflect.String:
		return v.Len() == 0
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if !isZero(v.Field(i)) {
				return false
			}
		}
		return true
	default:
		// This should never happens, but will act as a safeguard for
		// later, as a default value doesn't makes sense here.
		panic(fmt.Sprintf("Value.IsZero: %v", v.Kind()))
	}
}
