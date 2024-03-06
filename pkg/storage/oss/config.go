package oss

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"
)

//go:generate go run github.com/abice/go-enum -f=$GOFILE --marshal

// PlatformInfo
/**ENUM(
      INVALID,
      AWS,
	  LOCAL,
	  TENCENT,
	  BAIDU,
	  ORACLE,
	  MEMORY,
)
*/
type PlatformInfo int

// Config describes the basic information for build a CloudKV
type Config struct {
	Prefix     string
	Platform   string
	AccessKey  string `yaml:"access_key"`
	SecureKey  string `yaml:"secure_key"`
	BucketName string `yaml:"bucket_name"`
	Region     string
	Namespace  string // oracle required

	_platform PlatformInfo // temp variable
}

// Check will check credentials.
func (c *Config) Check() error {
	if c.BucketName == "" {
		return errors.New("missing bucket name")
	}

	if c._platform == PlatformInfoINVALID {
		if c.Platform == "" {
			return errors.New("missing platform, check your config")
		}
		platform, err := ParsePlatformInfo(strings.ToUpper(c.Platform))
		if err != nil {
			return err
		}
		c._platform = platform
	}

	switch c._platform {
	case PlatformInfoORACLE:
		if c.Namespace == "" {
			return errors.New("missing namespace field in oracle")
		}
	}
	if (c._platform != PlatformInfoLOCAL) && (c._platform != PlatformInfoMEMORY) {
		if c.AccessKey == "" {
			return errors.New("missing access key")
		}
		if c.SecureKey == "" {
			return errors.New("missing secure key")
		}
		if c.Region == "" {
			return errors.New("missing region")
		}
	}

	// try to check credentials
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fn, ok := GetFactory().Get(c._platform)
	if !ok {
		return fmt.Errorf("unknown platform :%s", c._platform.String())
	}
	if _, err := fn(ctx, *c); err != nil {
		return err
	}

	return nil
}

func (c *Config) Hash() (string, error) {
	first := sha256.New()
	for _, e := range []string{
		c.Platform,
		c.AccessKey,
		c.SecureKey,
		c.BucketName,
		c.Region,
	} {
		if _, err := first.Write([]byte(e)); err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("%x", first.Sum(nil)), nil
}

func (c *Config) toEndpoint() string {
	switch c._platform {
	case PlatformInfoAWS:
		return "s3.amazonaws.com"
	case PlatformInfoTENCENT:
		return fmt.Sprintf("cos.%s.myqcloud.com", c.Region)
	case PlatformInfoBAIDU:
		return fmt.Sprintf("s3.%s.bcebos.com", c.Region)
	case PlatformInfoORACLE:
		return fmt.Sprintf("%s.compat.objectstorage.%s.oraclecloud.com", c.Namespace, c.Region)
	}

	return ""
}

func (c *Config) effectiveBucket() string {
	return c.BucketName
}

func (c *Config) effectiveKey(key string) string {
	return fmt.Sprintf("%s/%s", c.Prefix, key)
}

func (c *Config) storageKey(key string) string {
	return path.Join(c.effectiveKey(""), key)
}
