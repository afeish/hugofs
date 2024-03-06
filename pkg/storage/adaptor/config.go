package adaptor

import (
	"bytes"
	"fmt"
	"io"
	"text/template"

	"github.com/pingcap/log"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

type (
	Config struct {
		ID        uint64
		MetaAddrs string `yaml:"meta_addrs"`
		DB        DbOption
		Test      TestOption
		// tcp
		IO IoOption
	}

	DbOption struct {
		Name string `yaml:"db_name"`
		Arg  string `yaml:"db_arg"`
	}

	TestOption struct {
		Enabled bool
		DBName  string `yaml:"db_name"`
		DBArg   string `yaml:"db_arg"`
	}
	IoOption struct {
		Transport TransportType `yaml:"transport"`
		BlockSize int           `yaml:"block_size"`
		Replica   int           `yaml:"replica"`
		Debug     bool          `yaml:"debug"`  // with debug mode enabled, no data will be saved to storage server
		Prefix    string        `yaml:"prefix"` // debug mode filepath prefix
	}
)

//go:generate go run github.com/abice/go-enum -f=$GOFILE --marshal

// PlatformInfo
/**ENUM(
      INVALID,
      GRPC,
	  TCP,
	  MOCK,
)
*/
type TransportType string

func NewConfig() *Config {
	return &Config{}
}

func NewConfigFromText(cfgText string) (*Config, error) {
	cfg := &Config{}
	err := yaml.Unmarshal([]byte(cfgText), cfg)
	return cfg, err
}

func (c *Config) GetTestAddr() string {
	return fmt.Sprintf("%s:%s", c.Test.DBName, c.Test.DBArg)
}

func NewConfigFromTemplate(text string, replacer map[string]string) (map[int]*Config, error) {
	return _newConfig(template.Must(template.New("").Parse(text)), replacer)
}

func NewConfigFromTemplateFile(cfgSource string, replacer map[string]string) (map[int]*Config, error) {
	temp := template.Must(template.ParseFiles(cfgSource))
	return _newConfig(temp, replacer)
}

func _newConfig(temp *template.Template, replacer map[string]string) (map[int]*Config, error) {
	var buf bytes.Buffer

	if err := temp.Execute(&buf, replacer); err != nil {
		return nil, errors.Wrap(err, "execute template")
	}

	var cfgMap = make(map[int]*Config)
	d := yaml.NewDecoder(&buf)
	for {
		// create new spec here
		spec := new(Config)
		// pass a reference to spec reference
		err := d.Decode(&spec)
		// check it was parsed
		if spec == nil {
			continue
		}
		// break the loop in case of EOF
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			log.Info("error parsing config", zap.Error(err))
			panic(err)
		}
		cfgMap[int(spec.ID)] = spec
	}
	return cfgMap, nil
}
