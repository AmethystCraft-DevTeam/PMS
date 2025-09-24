package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

type Config struct {
	Port            string
	Cookie          string
	RealIP          string
	Level           string
	NeteaseMusicAPI string
}

type SongURLResponse struct {
	Code int `json:"code"`
	Data []struct {
		ID            int         `json:"id"`
		URL           string      `json:"url"`
		Br            int         `json:"br"`
		Size          int         `json:"size"`
		MD5           string      `json:"md5"`
		Code          int         `json:"code"`
		Expi          int         `json:"expi"`
		Type          string      `json:"type"`
		Gain          float64     `json:"gain"`
		Peak          float64     `json:"peak"`
		Fee           int         `json:"fee"`
		Uf            interface{} `json:"uf"`
		Payed         int         `json:"payed"`
		Flag          int         `json:"flag"`
		CanExtend     bool        `json:"canExtend"`
		FreeTrialInfo interface{} `json:"freeTrialInfo"`
		Level         string      `json:"level"`
	} `json:"data"`
}

type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

var config Config

func init() {
	// 加载.env文件
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	config = Config{
		Port:            getEnvOrDefault("PORT", "8080"),
		Cookie:          getEnvOrDefault("NETEASE_COOKIE", ""),
		RealIP:          getEnvOrDefault("REAL_IP", "116.25.146.177"),
		Level:           getEnvOrDefault("LEVEL", "exhigh"),
		NeteaseMusicAPI: getEnvOrDefault("NETEASE_MUSIC_API", "https://example.com"),
	}

	// 检查必要的配置
	if config.Cookie == "" {
		log.Fatal("NETEASE_COOKIE is required in environment variables or .env file")
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	// 设置Gin模式
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	// 中间件
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"service":   "PublicMusicService",
			"version":   "1.0.0",
			"timestamp": time.Now().Unix(),
		})
	})

	// API路由 - 简化路径
	r.GET("/song", getSongURL)

	log.Printf("PublicMusicService (PMS) starting on port %s", config.Port)
	log.Printf("Netease Music API: %s", config.NeteaseMusicAPI)
	log.Printf("Default Level: %s", config.Level)

	if err := r.Run(":" + config.Port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

func getSongURL(c *gin.Context) {
	// 获取歌曲ID
	idStr := c.Query("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    400,
			Message: "Missing required parameter: id",
		})
		return
	}

	// 验证ID是否为有效数字
	songID, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    400,
			Message: "Invalid song id format",
		})
		return
	}

	// 获取可选参数
	level := c.DefaultQuery("level", config.Level)
	realIP := c.DefaultQuery("realip", config.RealIP)

	// 构建请求URL
	timestamp := time.Now().UnixNano() / 1e6 // 毫秒时间戳
	apiURL := fmt.Sprintf("%s/song/url/v1", config.NeteaseMusicAPI)

	// 构建查询参数
	params := url.Values{}
	params.Add("id", strconv.Itoa(songID))
	params.Add("level", level)
	params.Add("timestamp", strconv.FormatInt(timestamp, 10))
	params.Add("cookie", config.Cookie)
	params.Add("realIP", realIP)

	fullURL := fmt.Sprintf("%s?%s", apiURL, params.Encode())

	// 发起HTTP请求
	resp, err := http.Get(fullURL)
	if err != nil {
		log.Printf("Error requesting Netease API: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    500,
			Message: "Failed to request music service",
		})
		return
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    500,
			Message: "Failed to read response from music service",
		})
		return
	}

	// 解析JSON响应
	var songResp SongURLResponse
	if err := json.Unmarshal(body, &songResp); err != nil {
		log.Printf("Error parsing JSON response: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    500,
			Message: "Failed to parse response from music service",
		})
		return
	}

	// 检查网易云音乐API返回的状态码
	if songResp.Code != 200 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    songResp.Code,
			Message: "Music service returned error",
		})
		return
	}

	// 返回结果
	c.JSON(http.StatusOK, songResp)
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
