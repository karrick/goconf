package goconf

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	congomap "gopkg.in/karrick/congomap.v1"
)

// DefaultSectionName is the name of the default config section.  Any key-value pairs stored before
// any `[name]` blocks are stored in the default section name.
const DefaultSectionName = "General"

// Config is a data structure used to maintain an applications configuration.
type Config struct {
	pathname string
	cgm      congomap.Congomap
	ttl      time.Duration
}

// ConfigSetter is a function that mutates a new Config instance during instantiation.
type ConfigSetter func(*Config) error

// New returns a new Config data structure.
func New(pathname string, setters ...ConfigSetter) (*Config, error) {
	var err error

	c := &Config{pathname: pathname}

	for _, setter := range setters {
		if err := setter(c); err != nil {
			return nil, err
		}
	}

	cgms := []congomap.Setter{congomap.Lookup(c.lookupSection())}
	if c.ttl > 0 {
		cgms = append(cgms, congomap.TTL(c.ttl))
	}
	c.cgm, err = congomap.NewSyncAtomicMap(cgms...) // relatively few config sections
	if err != nil {
		return nil, err
	}

	return c, nil
}

// TTL mutates a new Config data structure to control how often values are refreshed.
func TTL(ttl time.Duration) func(*Config) error {
	return func(c *Config) error {
		if ttl <= 0 {
			return fmt.Errorf("ttl must be greater than 0")
		}
		c.ttl = ttl
		return nil
	}
}

func (c *Config) lookupSection() func(string) (interface{}, error) {
	return func(section string) (interface{}, error) {
		conf, err := parseConfigFile(c.pathname)
		if err != nil {
			return nil, err
		}
		sect, ok := conf[section]
		if !ok {
			return nil, fmt.Errorf("no such section: %q", section)
		}
		return sect, nil
	}
}

// Section returns a map of the key-value pairs for a specified section of the configuration file.
// The default section name is stored in `DefaultSectionName`.
func (c *Config) Section(section string) (map[string]string, error) {
	dict, err := c.cgm.LoadStore(section)
	if err != nil {
		return nil, err
	}
	return dict.(map[string]string), nil
}

// Close frees and releases resources consumed by Config data structure when no longer needed.
func (c *Config) Close() error {
	return c.cgm.Close()
}

func parseConfigFile(pathname string) (conf map[string]map[string]string, err error) {
	fh, err := os.Open(pathname)
	if err != nil {
		return
	}
	defer fh.Close()
	buf := bufio.NewScanner(fh)
	conf = make(map[string]map[string]string)
	section := DefaultSectionName
	sectionRe := regexp.MustCompile("^\\[([^\\]]+)\\]$")
	keyValRe := regexp.MustCompile("^([^=]+)\\s*=\\s*(.+)$")

	conf[section] = make(map[string]string) // always a default section

	for buf.Scan() {
		line := buf.Text()
		if comment := strings.IndexByte(line, ';'); comment >= 0 {
			line = line[:comment]
		}
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if md := sectionRe.FindStringSubmatch(line); md != nil {
			section = md[1]
		} else if md := keyValRe.FindStringSubmatch(line); md != nil {
			if conf[section] == nil {
				conf[section] = make(map[string]string)
			}
			conf[section][md[1]] = md[2]
		} else {
			err = fmt.Errorf("invalid config line: [%s]", line)
			return
		}
	}
	return
}
