package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	yaml "gopkg.in/yaml.v3"
)

var cwd string

func openbrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		log.Fatal(err)
	}

}

func removeNpmrc() error {
	npmrcPath := filepath.Join(cwd, ".npmrc")
	_, err := os.Stat(npmrcPath)
	if os.IsNotExist(err) {
		return err
	}

	if err := os.Remove(npmrcPath); err != nil {
		return err
	}

	return nil
}

type PackageJSON struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func readPackageJSON() (*PackageJSON, error) {

	jsonFilePath := filepath.Join(cwd, "package.json")
	file, err := os.Open(jsonFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open package.json file: %w", err)
	}
	defer file.Close()

	var packageJSON PackageJSON
	err = json.NewDecoder(file).Decode(&packageJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to parse package.json file: %w", err)
	}

	return &packageJSON, nil
}

type Config struct {
	VerdaccioURL string `yaml:"verdaccio_url"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	Email        string `yaml:"email"`
}

func main() {
	var err error
	cwd, err = os.Getwd()
	if err != nil {
		fmt.Printf("failed to get current working directory: %v\n", err)
		os.Exit(1)
	}

	configFile := filepath.Join(cwd, "config.yml")
	config, err := readConfig(configFile)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	err = checkNpmInstalled()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	err = npmLogin(config)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	err = npmPublish(config)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Package published to %s \n", config.VerdaccioURL)
	openbrowser(config.VerdaccioURL)
}

func readConfig(configFile string) (*Config, error) {
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func checkNpmInstalled() error {
	_, err := exec.LookPath("npm")
	if err != nil {
		return fmt.Errorf("npm not found in PATH")
	}
	return nil
}

func npmLogin(config *Config) error {
	removeErr := removeNpmrc()
	if removeErr != nil {
		return removeErr
	}
	authString := fmt.Sprintf("%s:%s", config.Username, config.Password)
	authToken := base64.StdEncoding.EncodeToString([]byte(authString))
	npmrcContent := fmt.Sprintf("registry=%s\n_auth=%s\nemail=%s\nalways-auth=true\n", config.VerdaccioURL, authToken, config.Email)
	npmrcPath := filepath.Join(cwd, ".npmrc")
	err := ioutil.WriteFile(npmrcPath, []byte(npmrcContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write .npmrc file: %w", err)
	}

	return nil
}

func npmPublish(config *Config) error {
	packageJSON, err := readPackageJSON()
	if err != nil {
		return err
	}

	packageName := packageJSON.Name
	packageVersion := packageJSON.Version

	if err := checkDuplicateVersion(config, packageName, packageVersion); err != nil {
		return err
	}

	cmd := exec.Command("npm", "publish", "--registry", config.VerdaccioURL)

	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

type PackageInfo struct {
	Versions map[string]Version `json:"versions"`
}

type Version struct {
	Version string `json:"version"`
}

func checkDuplicateVersion(config *Config, packageName string, version string) error {

	resp, err := http.Get(fmt.Sprintf("%s/%s", config.VerdaccioURL, packageName))
	if err != nil {
		return fmt.Errorf("failed to fetch package versions from server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch package versions from server: status code %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	packageInfo := PackageInfo{}
	err = json.Unmarshal(body, &packageInfo)

	if err != nil {
		return fmt.Errorf("failed to parse JSON response: %w", err)
	}

	for _, v := range packageInfo.Versions {
		if v.Version == version {
			return fmt.Errorf("version %s already exists in the repository, please modify the version number and try again", version)
		}
	}

	return nil
}
