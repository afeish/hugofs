package node

import (
	"os"
	"testing"
	"text/template"

	"github.com/stretchr/testify/require"
)

func TestParseConfig(t *testing.T) {
	str := `
id: 1
meta_addrs: 127.0.0.1:26166
block_size: 1024
section:
- id: 1
  engine: FS
  prefix: /tmp/phy1
  quota: 1024
  threhold: 0.75
db_name: badger
db_arg: /tmp/phy1/badger
`

	cfg, err := NewConfigFromText(str)
	require.NoError(t, err)
	require.NotNil(t, cfg)

}

func TestMultiParseConfig(t *testing.T) {
	str := `
id: 1
meta_addrs: 127.0.0.1:26166
block_size: 1024
section:
- id: 1
  engine: FS
  prefix: /tmp/phy1
  quota: 1024
  threhold: 0.75
db_name: badger
db_arg: /tmp/phy1/badger

---
id: 2
meta_addrs: 127.0.0.1:26167
block_size: 1024
section:
- id: 2
  engine: FS
  prefix: /tmp/phy2
  quota: 1024
  threhold: 0.75
db_name: badger
db_arg: /tmp/phy2/badger
`

	cfgMap, err := NewConfigFromTemplate(str, nil)
	require.NoError(t, err)
	require.Equal(t, 2, len(cfgMap))

}

func TestTemplateReplace(t *testing.T) {
	tempStr := `
hello {{ .name }}
	`
	temp, err := template.New("test").Parse(tempStr)
	require.NoError(t, err)

	m := make(map[string]string)
	m["name"] = "badger"
	err = temp.Execute(os.Stdout, m)
	require.NoError(t, err)
}
