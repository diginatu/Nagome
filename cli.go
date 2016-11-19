package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/diginatu/nagome/nicolive"
)

const (
	eventBufferSize  = 50
	accountFileName  = "account.yml"
	logFileName      = "info.log"
	pluginDirName    = "plugin"
	pluginConfigName = "plugin.yml"
	userDBFileName   = "user.sqlite"
	settingsFileName = "setting.yml"
)

// RunCli processes flags and io
func RunCli() {
	// set command line options
	flag.StringVar(&App.SavePath, "savepath", findUserConfigPath(), "Set <string> to save directory.")
	tcpPort := flag.String("p", "8025", `Port to wait TCP server for UI. (see uitcp)`)
	debugToStderr := flag.Bool("dbgtostd", false, `Output debug information to stderr.
	(in default, output to the log file in the save directory)`)
	printHelp := flag.Bool("help", false, "Print this help.")
	printHelp = flag.Bool("h", false, "Print this help. (shorthand)")
	printVersion := flag.Bool("v", false, "Print version information.")
	mkplug := flag.String("makeplug", "", "Make new plugin template with given name.")
	mainyml := flag.String("ymlmain", "", `specfy the config file of main plugin.
	Its format is same as yml file of normal plugins.`)
	mainyml = flag.String("y", "", `specfy the config file of main plugin. (shorthand)`)

	flag.Parse()

	log.SetFlags(log.Lshortfile | log.Ltime)

	err := App.SettingsSlots.Load()
	if err != nil {
		log.Println(err)
	}

	pluginPath := filepath.Join(App.SavePath, pluginDirName)

	if *printHelp {
		flag.Usage()
		return
	}
	if *printVersion {
		fmt.Println(App.Name, " ", App.Version)
		return
	}
	if *mkplug != "" {
		generatePluginTemplate(*mkplug, pluginPath)
		return
	}

	if err := os.MkdirAll(pluginPath, 0777); err != nil {
		log.Fatal("could not make save directory\n", err)
	}

	// set log
	var file *os.File
	if *debugToStderr {
		file = os.Stderr
	} else {
		var err error
		file, err = os.Create(filepath.Join(App.SavePath, logFileName))
		if err != nil {
			log.Fatal("could not open log file\n", err)
		}
	}
	defer file.Close()
	log.SetOutput(file)

	cv := NewCommentViewer(*tcpPort)

	// load account data
	ac := new(nicolive.Account)
	err = ac.Load(filepath.Join(App.SavePath, accountFileName))
	if err != nil {
		log.Println(err)
	} else {
		cv.Ac = ac
	}

	// add main plugin
	plug := newPlugin(cv)
	plug.Name = "main"
	plug.Description = "main plugin"
	plug.Version = "0.0"
	plug.Method = pluginMethodStd
	plug.Depends = []string{DomainNagome, DomainComment, DomainUI}
	if *mainyml != "" {
		if err != nil {
			log.Fatal(err)
		}
	}
	cv.AddPlugin(plug)
	if plug.Method == pluginMethodStd {
		plug.Open(&stdReadWriteCloser{os.Stdin, os.Stdout})
	}

	cv.Start()
	if cv.Ac != nil {
		cv.AntennaConnect()
	}
	cv.Wait()

	if cv.Settings.AutoSaveTo0Slot {
		App.SettingsSlots.Config[0] = &cv.Settings
	}
	err = App.SettingsSlots.Save()
	if err != nil {
		log.Fatalln(err)
	}
}

func generatePluginTemplate(name, pluginPath string) {
	p := filepath.Join(pluginPath, name)

	// check if the directory already exists
	_, err := os.Stat(p)
	if err == nil {
		log.Fatalln("Same name directory is already exists.")
	}

	if err := os.MkdirAll(p, 0777); err != nil {
		log.Fatalln("could not make save directory\n", err)
	}

	pl := plugin{
		Name:    name,
		Version: "1.0",
		Depends: []string{DomainNagome},
		Method:  "tcp",
		Exec:    []string{"{{path}}/" + name, "{{port}}", "{{no}}"},
	}
	pl.Save(filepath.Join(p, pluginConfigName))

	fmt.Printf("Create your plugin in : %s\n", p)
}
