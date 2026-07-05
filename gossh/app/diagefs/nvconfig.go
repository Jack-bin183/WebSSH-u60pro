package diagefs

import (
	"fmt"
	"strings"
)

const NVConfigPath = "/nvefs/nvconfig.ini"

type Client struct {
	LibPath string
}

type WriteResult struct {
	Path  string `json:"path"`
	Bytes int    `json:"bytes"`
	INI   string `json:"ini"`
	Par   string `json:"par"`
}

func (c *Client) Open() error {
	if err := diagOpen(c.LibPath); err != nil {
		return err
	}
	if err := diagInitDCI(); err != nil {
		diagClose()
		return err
	}
	return nil
}

func (c *Client) Close() {
	diagClose()
}

func (c *Client) WriteFile(path string, data []byte) error {
	return efsWriteFile(path, data)
}

func (c *Client) WriteNVConfig(digits string) (WriteResult, error) {
	ini, par, err := BuildNVConfigINI(digits)
	if err != nil {
		return WriteResult{}, err
	}
	if err := c.WriteFile(NVConfigPath, ini); err != nil {
		return WriteResult{}, err
	}
	return WriteResult{
		Path:  NVConfigPath,
		Bytes: len(ini),
		INI:   string(ini),
		Par:   par,
	}, nil
}

func BuildNVConfigINI(digits string) ([]byte, string, error) {
	if len(digits) != 15 {
		return nil, "", fmt.Errorf("串号必须是 15 位数字")
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return nil, "", fmt.Errorf("串号只能包含 0-9")
		}
	}

	parts := make([]string, 0, 9)
	parts = append(parts, "0x08")
	parts = append(parts, fmt.Sprintf("0x%cA", digits[0]))
	for i := 1; i < len(digits); i += 2 {
		parts = append(parts, fmt.Sprintf("0x%c%c", digits[i+1], digits[i]))
	}
	par := strings.Join(parts, ",")
	ini := fmt.Sprintf(`[NV_SYS0]
set=1
type=2
nvcode=550
len=9
par=%s
`, par)
	return []byte(ini), par, nil
}
