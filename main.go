package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
	"strconv"
)

type AurResult struct {
	Name  string  `json:"Name"`
	Votes int     `json:"Votes"`
}

type AurResponse struct {
	Results []AurResult `json:"results"`
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
			fmt.Println("Готово!")
		}
		return
	}

	if cmd == "top" {
		fmt.Println("🐳 Топ-10 тяжелых папок/пакетов в /usr:")
		runCommand("sh", "-c", "sudo du -sh /usr/lib/* /usr/share/* 2>/dev/null | sort -h | tail -n 10")
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
			fetchNews()
			runCommand("sudo", "pacman", "-Syyu", "--noconfirm")
		case "r":
			if len(os.Args) < pkgIndex+2 { return }
			runCommand("sudo", "pacman", "-Rns", os.Args[pkgIndex+1], "--noconfirm")
		case "s":
			if len(os.Args) < pkgIndex+2 { return }
			search(os.Args[pkgIndex+1], nodeps, autoYes)
		default:
			smartInstall(target, nodeps, autoYes)
	}
}

func fetchNews() {
	fmt.Println("🐧🗞️ Последние новости Arch Linux:")
	api := "https://archlinux.org" + "/feeds/news/"
	cmd := exec.Command("curl", "-s", api)
	out, _ := cmd.Output()
	lines := strings.Split(string(out), "\n")
	count := 0
	for _, line := range lines {
		if strings.Contains(line, "<title>") && count < 4 {
			title := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(line), "<title>"), "</title>")
			if count > 0 { fmt.Println("⚠️", title) }
			count++
		}
	}
	fmt.Println("-----------------")
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
			fmt.Println("Ken обновлен!")
			os.Exit(0)
		}
	}
}

func showHelp() {
	fmt.Println("🦭🗡️ Ken (ken/kn) - A Sharp Arch/AUR Helper")
	fmt.Println("\n[RU] Использование:")
	fmt.Println("  ken [пакет]     - Установить (авто-поиск + выбор)")
	fmt.Println("  ken u           - Обновить систему + новости")
	fmt.Println("  ken r [пакет]   - Удалить (пакет + зависимости + конфиги)")
	fmt.Println("  ken s [запрос]  - Поиск в AUR (по голосам)")
	fmt.Println("  ken top         - Показать 10 самых тяжелых пакетов")
	fmt.Println("  ken visudo      - Режим без пароля")
	fmt.Println("  ken y [пакет]   - Авто-установка (yes)")
	fmt.Println("  ken n [пакет]   - Установка без зависимостей")
	fmt.Println("\n[EN] Usage:")
	fmt.Println("  ken [pkg]       - Install (smart search + select)")
	fmt.Println("  ken u           - Update system + check news")
	fmt.Println("  ken r [pkg]     - Remove (Rns: package + deps + configs)")
	fmt.Println("  ken top         - List top 10 largest packages")
}

func smartInstall(pkg string, nodeps bool, autoYes bool) {
	if err := exec.Command("pacman", "-Si", pkg).Run(); err == nil {
		runCommand("sudo", "pacman", "-S", pkg, "--noconfirm")
		return
	}
	search(pkg, nodeps, autoYes)
}

func installFromAur(pkg string, nodeps bool, autoYes bool) {
	gitURL := "https://aur.archlinux.org" + "/" + pkg + ".git"
	tmpDir := "/tmp/ken-" + pkg
	os.RemoveAll(tmpDir)
	if err := runCommand("git", "clone", "--depth=1", gitURL, tmpDir); err != nil { return }
	os.Chdir(tmpDir)

	mkArgs := []string{"-si", "--noconfirm"}
	if nodeps { mkArgs = append(mkArgs, "-d") }

	if err := runCommand("makepkg", mkArgs...); err != nil && !nodeps {
		out, _ := exec.Command("makepkg", "--printsrcinfo").Output()
		lines := strings.Split(string(out), "\n")
		var deps []string
		for _, line := range lines {
			if strings.Contains(line, "depends = ") {
				parts := strings.Split(line, "=")
				if len(parts) > 1 {
					d := strings.TrimSpace(parts[1])
					deps = append(deps, strings.Fields(d)[0])
				}
			}
		}

		if len(deps) > 0 {
			answer := "y"
			if !autoYes {
				fmt.Printf("\n🐧Нужны зависимости: %s. Ставим? [y/N]: ", strings.Join(deps, ", "))
				fmt.Scanln(&answer)
			}
			if strings.ToLower(answer) == "y" {
				for _, d := range deps {
					smartInstall(d, false, autoYes)
				}
				os.Chdir(tmpDir)
				runCommand("makepkg", mkArgs...)
			}
		}
	}
	os.Chdir("/")
	os.RemoveAll(tmpDir)
	fmt.Println("🧹 Подметаем за собой...")
	runCommand("sudo", "pacman", "-Sc", "--noconfirm")
}

func search(q string, nodeps bool, autoYes bool) {
	searchURL := "https://aur.archlinux.org/rpc/?v=5&type=search&arg=" + q
	r, _ := http.Get(searchURL)
	var aur AurResponse
	json.NewDecoder(r.Body).Decode(&aur)

	sort.Slice(aur.Results, func(i, j int) bool {
		return aur.Results[i].Votes > aur.Results[j].Votes
	})

	if len(aur.Results) == 0 {
		fmt.Println("🦭 Ничего не найдено.")
		return
	}

	limit := len(aur.Results)
	if limit > 15 { limit = 15 }

	fmt.Println("🍶 Найденные пакеты (отсортировано по Votes):")
	for i := 0; i < limit; i++ {
		fmt.Printf("%d) %s (%d ⭐)\n", i+1, aur.Results[i].Name, aur.Results[i].Votes)
	}

	fmt.Print("\n🗡️ Выбери номера через пробел: ")
	var input string
	fmt.Scanln(&input)
	if strings.TrimSpace(input) == "" { return }

	choices := strings.Fields(input)
	for _, c := range choices {
		idx, err := strconv.Atoi(c)
		if err == nil && idx > 0 && idx <= limit {
			installFromAur(aur.Results[idx-1].Name, nodeps, autoYes)
		}
	}
}

func runCommand(name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Stdout, c.Stderr, c.Stdin = os.Stdout, os.Stderr, os.Stdin
	return c.Run()
}
