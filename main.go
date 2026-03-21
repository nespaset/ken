package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type AurResponse struct {
	Results []struct {
		Name string `json:"Name"`
	} `json:"results"`
}

func main() {
	if len(os.Args) < 2 {
		showHelp()
		return
	}

	cmd := os.Args[1]

	if cmd == "h" || cmd == "help" {
		showHelp()
		return
	}

	if cmd == "visudo" {
		fmt.Println("🦭 Настройка беспарольного режима для Ken...")
		user := os.Getenv("USER")
		rule := fmt.Sprintf("%s ALL=(ALL) NOPASSWD: /usr/bin/pacman, /usr/bin/makepkg\n", user)
		echoCmd := exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | sudo tee /etc/sudoers.d/ken", rule))
		echoCmd.Stdout, echoCmd.Stderr, echoCmd.Stdin = os.Stdout, os.Stderr, os.Stdin
		if err := echoCmd.Run(); err == nil {
			fmt.Println("Готово! Меч Ken больше не требует пароля.")
		}
		return
	}

	checkUpdate("nespaset")

	exec.Command("sudo", "-v").Run()

	nodeps := false
	autoYes := false
	pkgIndex := 1

	if len(os.Args) > 2 {
		if cmd == "y" {
			autoYes = true
			pkgIndex = 2
		} else if cmd == "n" {
			nodeps = true
			pkgIndex = 2
		} else if cmd == "yn" || cmd == "ny" {
			autoYes = true
			nodeps = true
			pkgIndex = 2
		}
	}

	target := os.Args[pkgIndex]

	switch target {
		case "u":
			runCommand("sudo", "pacman", "-Syyu", "--noconfirm")
		case "r":
			if len(os.Args) < pkgIndex+2 { return }
			runCommand("sudo", "pacman", "-Rns", os.Args[pkgIndex+1], "--noconfirm")
		case "s":
			if len(os.Args) < pkgIndex+2 { return }
			search(os.Args[pkgIndex+1])
		default:
			installPackage(target, nodeps, autoYes)
	}
}

func checkUpdate(user string) {
	apiURL := "https://api.github.com" + "/repos/nespaset/ken/commits/main"
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil { return }
	defer resp.Body.Close()

	var data struct {
		SHA    string `json:"sha"`
		Commit struct { Message string `json:"message"` } `json:"commit"`
	}
	json.NewDecoder(resp.Body).Decode(&data)

	home, _ := os.UserHomeDir()
	configDir := home + "/.config/ken"
	lastSHAFile := configDir + "/last_commit"
	oldSHA, _ := os.ReadFile(lastSHAFile)

	if string(oldSHA) != data.SHA && strings.Contains(strings.ToLower(data.Commit.Message), "update") {
		fmt.Printf("🗡️ Новая заточка Ken: %s\nОбновиться? [y/N]: ", data.Commit.Message)
		var choice string
		fmt.Scanln(&choice)
		if strings.ToLower(choice) == "y" {
			fmt.Println("🗡️ Перековка Ken...")
			tmp := "/tmp/ken-upgrade"
			os.RemoveAll(tmp)
			gitURL := "https://github.com" + "/nespaset/ken"
			runCommand("git", "clone", gitURL, tmp)
			os.Chdir(tmp)
			runCommand("go", "build", "-o", "ken", "main.go")
			runCommand("sudo", "mv", "ken", "/usr/local/bin/ken")
			runCommand("sudo", "ln", "-sf", "/usr/local/bin/ken", "/usr/local/bin/kn")

			os.MkdirAll(configDir, 0755)
			os.WriteFile(lastSHAFile, []byte(data.SHA), 0644)

			fmt.Println("Ken обновлен! Используйте ken или kn.")
			os.Exit(0)
		}
	}
}

func showHelp() {
	fmt.Println("🦭🗡️ Ken (ken/kn) - A Sharp Arch/AUR Helper")
	fmt.Println("\n[RU] Использование:")
	fmt.Println("  ken [пакет]     - Установить")
	fmt.Println("  ken u           - Обновить систему")
	fmt.Println("  ken visudo      - Режим без пароля")
	fmt.Println("  ken y [пакет]   - Авто-установка")
	fmt.Println("  ken n [пакет]   - Без зависимостей")
}

func installPackage(pkg string, nodeps bool, autoYes bool) {
	if err := exec.Command("pacman", "-Si", pkg).Run(); err == nil {
		if err := runCommand("sudo", "pacman", "-S", pkg, "--noconfirm"); err != nil {
			runCommand("sudo", "pacman", "-Syyu", "--noconfirm")
			runCommand("sudo", "pacman", "-S", pkg, "--noconfirm")
		}
		return
	}

	aurURL := "https://aur.archlinux.org/rpc/?v=5&type=info&arg[]=" + pkg
	resp, _ := http.Get(aurURL)
	defer resp.Body.Close()
	var aur AurResponse
	json.NewDecoder(resp.Body).Decode(&aur)

	if len(aur.Results) == 0 {
		search(pkg)
		return
	}

	gitURL := "https://aur.archlinux.org" + "/" + pkg + ".git"
	tmpDir := "/tmp/ken-" + pkg
	os.RemoveAll(tmpDir)
	runCommand("git", "clone", "--depth=1", gitURL, tmpDir)
	os.Chdir(tmpDir)

	mkArgs := []string{"-si", "--noconfirm"}
	if nodeps { mkArgs = append(mkArgs, "-d") }

	if err := runCommand("makepkg", mkArgs...); err != nil && !nodeps {
		out, _ := exec.Command("makepkg", "--printsrcinfo").Output()
		lines := strings.Split(string(out), "\n")
		var deps []string
		for _, line := range lines {
			if strings.Contains(line, "depends = ") {
				d := strings.TrimSpace(strings.Split(line, "=")[1])
				deps = append(deps, strings.Fields(d)[0])
			}
		}

		if len(deps) > 0 {
			answer := "y"
			if !autoYes {
				fmt.Printf("\nНужны зависимости: %s. Ставим? [y/N]: ", strings.Join(deps, ", "))
				fmt.Scanln(&answer)
			}
			if strings.ToLower(answer) == "y" {
				for _, d := range deps {
					installPackage(d, false, autoYes)
				}
				os.Chdir(tmpDir)
				runCommand("makepkg", mkArgs...)
			}
		}
	}
	os.Chdir("/")
	os.RemoveAll(tmpDir)
}

func search(q string) {
	searchURL := "https://aur.archlinux.org/rpc/?v=5&type=search&arg=" + q
	r, _ := http.Get(searchURL)
	var aur AurResponse
	json.NewDecoder(r.Body).Decode(&aur)
	for _, res := range aur.Results { fmt.Println("📦", res.Name) }
}

func runCommand(name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Stdout, c.Stderr, c.Stdin = os.Stdout, os.Stderr, os.Stdin
	return c.Run()
}
