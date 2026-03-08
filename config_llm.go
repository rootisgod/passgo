// config_llm.go - LLM configuration loading from ~/.passgo/llm.conf
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LLMConfig holds the LLM integration settings.
type LLMConfig struct {
	BaseURL   string
	APIKey    string
	Model     string
	MCPBinary string
}

const llmConfigFile = "llm.conf"

// defaultLLMConfig returns the default configuration.
func defaultLLMConfig() LLMConfig {
	return LLMConfig{
		BaseURL: DefaultLLMBaseURL,
		Model:   DefaultLLMModel,
	}
}

// llmConfigPath returns the full path to the LLM config file.
func llmConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".passgo", llmConfigFile), nil
}

// loadLLMConfig reads the LLM config from ~/.passgo/llm.conf.
// Returns default config if the file doesn't exist.
func loadLLMConfig() (LLMConfig, error) {
	cfg := defaultLLMConfig()

	path, err := llmConfigPath()
	if err != nil {
		return cfg, err
	}

	f, err := os.Open(path) // #nosec G304 -- path from UserHomeDir
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)

		switch key {
		case "base-url":
			if val != "" {
				cfg.BaseURL = val
			}
		case "api-key":
			cfg.APIKey = val
		case "model":
			if val != "" {
				cfg.Model = val
			}
		case "mcp-binary":
			cfg.MCPBinary = val
		}
	}

	return cfg, scanner.Err()
}

// saveLLMConfig writes the config to ~/.passgo/llm.conf.
func saveLLMConfig(cfg LLMConfig) error {
	path, err := llmConfigPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}

	content := fmt.Sprintf(`# PassGo LLM Configuration
# Any OpenAI-compatible endpoint works (OpenRouter, Ollama, OpenAI, LiteLLM, etc.)
base-url=%s
api-key=%s
model=%s
mcp-binary=%s
`, cfg.BaseURL, cfg.APIKey, cfg.Model, cfg.MCPBinary)

	return os.WriteFile(path, []byte(content), 0o600)
}
