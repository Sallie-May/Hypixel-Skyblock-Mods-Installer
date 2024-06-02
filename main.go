package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"

	"os"
	"path/filepath"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type Mod struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Tags   string `json:"tags"`
	GITHUB string `json:"github_url"`
}

func loadMods(filename string) ([]Mod, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var mods []Mod
	if err := json.Unmarshal(bytes, &mods); err != nil {
		return nil, err
	}

	return mods, nil
}

type Release struct {
	Name   string  `json:"name"`
	Assets []Asset `json:"assets"`
}
type Asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

func downloadFile(urls, name, outputPath string, label *widget.Label, w fyne.Window) {
	if strings.Contains(urls, "odinclient") {
		dialog.ShowError(fmt.Errorf("error downloading file | Cheat Version of Odin"), w)
		return
	}

	req, err := http.NewRequest("GET", urls, nil)
	if err != nil {
		dialog.ShowError(fmt.Errorf("error creating request: %w", err), w)
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br, zstd")
	req.Header.Set("Accept-Language", "fr-FR,fr;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Cache-Control", "no-cache")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		dialog.ShowError(fmt.Errorf("error downloading file: %w", err), w)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		dialog.ShowError(fmt.Errorf("error downloading file: %s", resp.Status), w)
		return
	}

	fileName := name + ".jar"

	out, err := os.Create(filepath.Join(outputPath, fileName))
	if err != nil {
		dialog.ShowError(fmt.Errorf("error creating file: %w", err), w)
		return
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		dialog.ShowError(fmt.Errorf("error saving file: %w", err), w)
		return
	}

	label.SetText(fmt.Sprintf("%s (Successfully downloaded)", name))
}
func searchMods(searchText string, selectedTags []string, mods []Mod) []Mod {
	if searchText == "" && len(selectedTags) == 0 {
		return mods
	}

	var filteredMods []Mod
	addedMods := make(map[string]bool)

	for _, mod := range mods {
		if strings.Contains(strings.ToLower(mod.Name), strings.ToLower(searchText)) {
			if !addedMods[mod.Name] {
				filteredMods = append(filteredMods, mod)
				addedMods[mod.Name] = true
			}
			continue
		}
		for _, tag := range strings.Split(mod.Tags, "|") {
			for _, selectedTag := range selectedTags {
				if strings.EqualFold(tag, selectedTag) {
					if !addedMods[mod.Name] {
						filteredMods = append(filteredMods, mod)
						addedMods[mod.Name] = true
					}
					break
				}
			}
		}
	}
	return filteredMods
}

func getLatestRelease(repoURL string) (*Release, error) {
	parts := strings.Split(repoURL, "/")
	owner := parts[len(parts)-2]
	repo := parts[len(parts)-1]
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch latest release: %s", resp.Status)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

func main() {

	a := app.New()
	a.Settings().SetTheme(theme.DarkTheme())

	w := a.NewWindow("Hypixel Skyblock Mods Installer")

	mods, err := loadMods("mods.json")
	if err != nil {
		fmt.Println("error loading mods:", err)
		return
	}

	vbox := container.NewVBox()

	title := canvas.NewText("Hypixel Skyblock Mods Installer", color.White)
	title.TextSize = 24
	title.Alignment = fyne.TextAlignCenter
	titleContainer := container.NewVBox(title, widget.NewSeparator())
	vbox.Add(titleContainer)

	getUniqueTags := func() []string {
		tagMap := make(map[string]bool)
		for _, mod := range mods {
			tags := strings.Split(mod.Tags, "|")
			for _, tag := range tags {
				tagMap[tag] = true
			}
		}
		uniqueTags := make([]string, 0, len(tagMap)-1)
		for tag := range tagMap {
			uniqueTags = append(uniqueTags, tag)
		}
		return uniqueTags
	}

	var tagSlider *widget.CheckGroup

	defaultModPath := "No path selected"
	modPathEntry := widget.NewEntry()
	modPathEntry.SetPlaceHolder("Enter mod path")
	modPathEntry.Hide()

	modPathButton := widget.NewButton("Select Folder", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err == nil && uri != nil {
				modPath := uri.Path()
				modPathEntry.SetText(modPath)
				modPathEntry.Show()
			}
		}, w)
	})

	list := container.NewVBox()

	updateList := func(mods []Mod) {
		list.Objects = nil
		for _, mod := range mods {
			mod := mod
			label := widget.NewLabel(mod.Name)

			githubButton := widget.NewButton("Official Website", func() {
				if mod.GITHUB != "" {
					githubURL, err := url.Parse(mod.URL)
					if err != nil {
						fmt.Println("error parsing GitHub URL:", err)
						return
					}
					if err := fyne.CurrentApp().OpenURL(githubURL); err != nil {
						fmt.Println("error opening URL:", err)
					}
				} else {
					fmt.Println("GitHub URL not provided for:", mod.Name)
				}
			})

			githubButton.Importance = widget.LowImportance

			downloadButton := widget.NewButton("Download", func() {
				if modPathEntry.Text == "" {
					dialog.ShowError(fmt.Errorf("Mod path is empty"), w)
					return
				}

				var downloadURL string
				if strings.Contains(mod.URL, "github.com") {
					release, err := getLatestRelease(mod.GITHUB)
					if err != nil {
						dialog.ShowError(fmt.Errorf("error fetching latest release: %w", err), w)
						return
					}
					for _, asset := range release.Assets {
						if strings.HasSuffix(asset.Name, ".jar") {
							downloadURL = asset.DownloadURL
							break
						}
					}
					if downloadURL == "" {
						dialog.ShowError(fmt.Errorf("no .jar file found in the latest release"), w)
						return
					}
				} else {
					downloadURL = mod.GITHUB
				}

				label.SetText(fmt.Sprintf("%s (Downloading...)", mod.Name))

				go func() {
					downloadFile(downloadURL, mod.Name, modPathEntry.Text, label, w)
				}()
			})

			downloadButton.Importance = widget.HighImportance

			hbox := container.NewHBox(
				label,
				layout.NewSpacer(),
				githubButton,
				downloadButton,
			)
			list.Add(hbox)
		}
		list.Refresh()
	}

	updateList(mods)

	scroll := container.NewScroll(list)
	scroll.SetMinSize(fyne.NewSize(300, 200))
	var pathLabel *widget.Label
	x := 0
	for range mods {
		x++
	}

	contributeButton := widget.NewButton("Contribute", func() {
		u, err := url.Parse("https://github.com/AdvancedSkyblock/Hypixel-Skyblock-Mods-Installer/issues/new?assignees=&labels=&projects=&template=feature_request.md&title=Contribution")
		if err != nil {
			return
		}

		a.OpenURL(u)
	})
	GithubRepo := widget.NewButton("Github Repository", func() {
		u, err := url.Parse("https://github.com/AdvancedSkyblock/Hypixel-Skyblock-Mods-Installer/")
		if err != nil {
			return
		}

		a.OpenURL(u)
	})

	issuesButton := widget.NewButton("Open a new issue", func() {
		u, err := url.Parse("https://github.com/AdvancedSkyblock/Hypixel-Skyblock-Mods-Installer/issues/new?assignees=&labels=&projects=&template=bug_report.md&title=Bug Report")
		if err != nil {
			return
		}

		a.OpenURL(u)
	})

	buttonsContainer := container.NewHBox(
		contributeButton,
		issuesButton,
		GithubRepo,
	)

	pathLabel = widget.NewLabel("Loaded mods : " + strconv.Itoa(x))
	pathLabel.Alignment = fyne.TextAlignCenter

	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Search for a mod")
	searchEntry.SetText("")
	searchEntry.OnChanged = func(text string) {
		selected := strings.Join(strings.Split(text, " "), " ")
		filteredMods := searchMods(selected, tagSlider.Selected, mods)
		updateList(filteredMods)
	}

	autoDetermineButton := widget.NewButton("Automatically Determine Path", func() {
		defaultModPath = filepath.Join(os.Getenv("APPDATA"), ".minecraft", "mods", "1.8.9")
		if _, err := os.Stat(defaultModPath); os.IsNotExist(err) {
			defaultModPath = filepath.Join(os.Getenv("APPDATA"), ".minecraft", "mods")
			if _, err := os.Stat(defaultModPath); os.IsNotExist(err) {
				defaultModPath = ""
			}
		}

		if defaultModPath != "" {
			modPathEntry.SetText(defaultModPath)
		}
	})

	updateTagSlider := func() {
		if tagSlider != nil {
			vbox.Remove(tagSlider)
		}
		tags := getUniqueTags()
		tagSlider = widget.NewCheckGroup(tags, func(selected []string) {
			searchText := strings.Join(selected, " ")
			filteredMods := searchMods(searchText, tagSlider.Selected, mods)
			updateList(filteredMods)
			if len(selected) > 0 {
				autoDetermineButton.Enable()
			} else {
				autoDetermineButton.Enable()
			}
		})

		tagContainer := container.NewGridWithColumns(3)
		for _, tag := range tags {
			tagContainer.Add(widget.NewCheck(tag, func(checked bool) {
			}))
		}
		vbox.Add(tagContainer)
	}
	updateTagSlider()

	buttonContainer := container.NewVBox(
		buttonsContainer,
		pathLabel,
		modPathEntry,
		modPathButton,
		autoDetermineButton,
		searchEntry,
		tagSlider,
	)

	vbox.Add(buttonContainer)
	w.SetContent(container.NewVBox(
		titleContainer,
		buttonContainer,
		scroll,
	))
	searchEntry.SetText("")
	searchEntry.Show()
	modPathEntry.Show()
	w.Resize(fyne.NewSize(1280, 720))

	w.ShowAndRun()
}
