package utils

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func Check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func IsDebugEnabled() bool {
	config, err := LoadConfiguration("/etc/skypier/config.json")
	if err != nil {
		log.Fatal(err)
		return true
	} else {
		return config.Debug
	}
}

// Nickname			 godoc
// @Summary      Get the node nickname
// @Description  Get the node nickname
// @Tags         VPN
// @Produce      json
// @Success      200
// @Router       /nickname [get]
func Nickname(c *gin.Context) {
	config, err := LoadConfiguration("/etc/skypier/config.json")
	if err != nil {
		log.Fatal(err)
	} else {
		c.IndentedJSON(200, config.Nickname)
	}
}

// Ping			 godoc
// @Summary      Ping the node locally from the UI
// @Description  Ping the node locally from the UI
// @Tags         VPN
// @Produce      json
// @Success      200
// @Router       /ping [get]
func Ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "pong",
	})
}

// Ok			 godoc
// @Summary      Default root API message
// @Description  Default root API message
// @Tags         VPN
// @Produce      json
// @Success      200
// @Router       / [get]
func Ok(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "OK",
	})
}
