package node

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"text/template"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	"github.com/afeish/hugo/pkg/storage/adaptor"
	"github.com/afeish/hugo/pkg/storage/engine"
	"github.com/afeish/hugo/pkg/storage/oss"
	"github.com/afeish/hugo/pkg/util/size"
	"github.com/jinzhu/copier"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

var (
	ErrInvalidHostname = errors.New("invalid hostname")
)

type VolCfg struct {
	ID       PhysicalVolumeID
	Engine   engine.EngineType
	Prefix   string
	Quota    *size.SizeSuffix
	Threhold *float64
}

type Config struct {
	ID        uint64
	IP        string
	Port      uint16
	TcpPort   uint16
	MetaAddrs string           `yaml:"meta_addrs"`
	BlockSize *size.SizeSuffix `yaml:"block_size"`
	Section   []VolCfg
	DB        adaptor.DbOption
	Test      adaptor.TestOption

	Object oss.Config

	Lg *zap.Logger
}

func NewConfig() *Config {
	return &Config{}
}

func NewConfigFromText(cfgText string) (*Config, error) {
	cfg := &Config{}
	err := yaml.Unmarshal([]byte(cfgText), cfg)
	cfg.postConstruct()
	return cfg, err
}

func NewConfigFromFile(cfgSource string) (map[int]*Config, error) {
	f, err := os.Open(cfgSource)
	if err != nil {
		return nil, errors.Wrap(err, "err read config")
	}
	var cfgMap = make(map[int]*Config)
	d := yaml.NewDecoder(f)
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
			panic(err)
		}
		spec.postConstruct()
		cfgMap[int(spec.ID)] = spec
	}
	return cfgMap, nil
}
func NewConfigFromTemplate(text string, replacer map[string]string) (map[int]*Config, error) {
	return _newConfig(template.Must(template.New("").Parse(text)), replacer)
}

func NewConfigFromTemplateFile(cfgSource string, replacer map[string]string) (map[int]*Config, error) {
	return _newConfig(template.Must(template.ParseFiles(cfgSource)), replacer)
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
			panic(err)
		}
		spec.postConstruct()
		cfgMap[int(spec.ID)] = spec
	}
	return cfgMap, nil
}

func (c *Config) postConstruct() {
	for _, sec := range c.Section {
		if sec.Engine == "" {
			sec.Engine = engine.EngineTypeFS
		}
	}
}

func (c *Config) Must() error {
	if c.Test.Enabled {
		return nil
	}
	addr := net.ParseIP(c.IP)
	if addr == nil {
		addrs, err := net.LookupHost(c.IP)
		if err != nil {
			return errors.Wrap(err, "invalid hostname")
		}
		if len(addrs) == 0 {
			return ErrInvalidHostname
		}
	}
	if c.Port == 0 {
		return errors.New("must provide a valid port")
	}
	return nil
}

func (c *Config) GetAddr() string {
	return fmt.Sprintf("%s:%d", c.IP, c.Port)
}

func (c *Config) ToAdaptorConfig() *adaptor.Config {
	var d adaptor.Config
	if err := copier.Copy(&d, c); err != nil {
		return nil
	}
	return &d
}
