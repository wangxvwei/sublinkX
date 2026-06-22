package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"sublink/middlewares"
	"sublink/models"
	"sublink/routers"
	"sublink/settings"
	"sublink/utils"

	"github.com/gin-gonic/gin"
)

//go:embed static/js/*
//go:embed static/css/*
//go:embed static/img/*
//go:embed static/*
var embeddedFiles embed.FS

//go:embed template
var Template embed.FS

var version = "2.2.0-latest"

func Templateinit() {
	subFS, err := fs.Sub(Template, "template")
	if err != nil {
		log.Println(err)
		return
	}

	entries, err := fs.ReadDir(subFS, ".")
	if err != nil {
		log.Println(err)
		return
	}

	if _, err = os.Stat("./template"); os.IsNotExist(err) {
		err = os.Mkdir("./template", 0755)
		if err != nil {
			log.Println(err)
			return
		}
	}

	for _, entry := range entries {
		if _, err := os.Stat("./template/" + entry.Name()); !os.IsNotExist(err) {
			continue
		}

		data, err := fs.ReadFile(subFS, entry.Name())
		if err != nil {
			log.Println(err)
			continue
		}

		err = os.WriteFile("./template/"+entry.Name(), data, 0666)
		if err != nil {
			log.Println(err)
		}
	}
}

func main() {
	models.ConfigInit()
	config := models.ReadConfig()
	port := config.Port

	var Isversion bool
	flag.BoolVar(&Isversion, "version", false, "show version")
	flag.Parse()
	if Isversion {
		fmt.Println(version)
		return
	}

	models.InitSqlite()
	args := os.Args
	if len(args) < 2 {
		Run(port)
		return
	}

	settingCmd := flag.NewFlagSet("setting", flag.ExitOnError)
	var username, password string
	settingCmd.StringVar(&username, "username", "", "set username")
	settingCmd.StringVar(&password, "password", "", "set password")
	settingCmd.IntVar(&port, "port", 8000, "set listen port")

	switch args[1] {
	case "setting":
		settingCmd.Parse(args[2:])
		fmt.Println(username, password)
		settings.ResetUser(username, password)
	case "run":
		settingCmd.Parse(args[2:])
		models.SetConfig(models.Config{
			Port: port,
		})
		Run(port)
	}
}

func Run(port int) {
	r := gin.Default()
	utils.Loginit()
	Templateinit()

	r.Use(middlewares.AuthorToken)

	staticFiles, err := fs.Sub(embeddedFiles, "static")
	if err != nil {
		log.Println(err)
	}
	r.StaticFS("/static", http.FS(staticFiles))
	r.GET("/", func(c *gin.Context) {
		data, err := fs.ReadFile(staticFiles, "index.html")
		if err != nil {
			c.Error(err)
			return
		}
		c.Data(200, "text/html", data)
	})

	routers.User(r)
	routers.Mentus(r)
	routers.Subcription(r)
	routers.Nodes(r)
	routers.Clients(r)
	routers.Total(r)
	routers.Templates(r)
	routers.Version(r, version)
	routers.Update(r, version)

	r.Run(fmt.Sprintf("0.0.0.0:%d", port))
}
