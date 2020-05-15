package postapi

import (
	"runtime"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/gogap/config"
	"github.com/sirupsen/logrus"
)

func (p *PostAPI) loadCORS(router *gin.Engine, conf config.Configuration) {

	var corsConf cors.Config
	if conf.IsEmpty() {
		corsConf = cors.DefaultConfig()
		corsConf.AllowMethods = []string{"POST"}
		corsConf.AllowOrigins = []string{"*"}
		corsConf.AllowOriginFunc = func(origin string) bool {
			return true
		}

		logrus.WithField("component", "postapi").WithField("alias", p.alias).Infoln("using default cors config")
	} else {
		corsConf = cors.Config{
			AllowOrigins:     conf.GetStringList("allow-origins"),
			AllowMethods:     conf.GetStringList("allow-methods"),
			AllowHeaders:     conf.GetStringList("allow-headers"),
			ExposeHeaders:    conf.GetStringList("expose-headers"),
			AllowCredentials: conf.GetBoolean("allow-credentials", false),
			MaxAge:           conf.GetTimeDuration("max-age", time.Hour*12),
		}

		corsConf.AllowOriginFunc = wildcardMatchFunc(corsConf.AllowOrigins)
	}

	corsConf.AllowHeaders = append(corsConf.AllowHeaders, "X-Api", "X-Api-Batch", "X-Api-Timeout")

	router.Use(cors.New(corsConf))
}

func (p *PostAPI) loadPprof(router *gin.Engine, conf config.Configuration) {

	if conf == nil {
		return
	}

	if !conf.GetBoolean("enabled", false) {
		return
	}

	logrus.WithField("component", "postapi").WithField("alias", p.alias).Infoln("http.pprof enabled")

	pprof.Register(router)
	runtime.SetBlockProfileRate(int(conf.GetInt32("block-profile-rate", 0)))
}

func (p *PostAPI) loadGZip(router *gin.Engine, conf config.Configuration) {

	if conf == nil {
		return
	}

	if !conf.GetBoolean("enabled", true) {
		return
	}

	logrus.WithField("component", "postapi").WithField("alias", p.alias).Infoln("gzip enabled")

	compressLevel := conf.GetString("level", "default")

	level := gzip.DefaultCompression

	switch compressLevel {
	case "best-compression":
		{
			level = gzip.BestCompression
		}
	case "best-speed":
		{
			level = gzip.BestSpeed
		}
	}

	router.Use(gzip.Gzip(level))
}
