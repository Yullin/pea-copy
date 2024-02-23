package main

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
)

var (
	db *sql.DB
	mu sync.Mutex
)
var (
	uploadPath string = "./uploads"
	dbPath     string = "./submissions.db"
)

type Submission struct {
	ID         int
	Content    string
	FilePath   string
	SubmitTime time.Time
}

func GetDB() *sql.DB {
	mu.Lock()
	defer mu.Unlock()

	if db == nil {
		var err error
		db, err = sql.Open("sqlite3", dbPath)
		if err != nil {
			log.Fatal(err)
		}

		// 检查数据库表是否存在，如果不存在则创建
		_, err = db.Exec(`CREATE TABLE IF NOT EXISTS submissions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			content TEXT,
			filePath TEXT,
			submitTime DATETIME
		)`)
		if err != nil {
			log.Fatal(err)
		}
	}

	return db
}

func InsertSubmission(content, filePath string) error {
	db := GetDB()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("INSERT INTO submissions(content, filePath, submitTime) VALUES(?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(content, filePath, time.Now())
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func QuerySubmissions() ([]Submission, error) {
	db := GetDB()
	rows, err := db.Query("SELECT id, content, filePath, submitTime FROM submissions ORDER BY submitTime DESC LIMIT 10")
	if err != nil {
		log.Println("Query sqlite error: ", err)
		return nil, err
	}
	defer rows.Close()

	var submissions []Submission
	for rows.Next() {
		var s Submission
		err = rows.Scan(&s.ID, &s.Content, &s.FilePath, &s.SubmitTime)
		if err != nil {
			return nil, err
		}
		submissions = append(submissions, s)
	}
	log.Println("submissions in QuerySubmissions functions: ", submissions)
	return submissions, nil
}

func main() {
	if _, err := os.Stat(uploadPath); os.IsNotExist(err) {
		// 目录不存在，创建目录
		err := os.MkdirAll(uploadPath, os.ModePerm)
		if err != nil {
			fmt.Println("Direcotry Create Failed:", err)
		} else {
			fmt.Println("Directory Created")
		}
	} else {
		fmt.Println("Directory Exist")
	}

	r := gin.Default()

	r.LoadHTMLGlob("templates/*")

	//r.POST("/submit", submitHandler)
	r.POST("/submit", func(c *gin.Context) {
		textInput := c.PostForm("text")
		log.Println("Get Input Text: ", textInput)

		file, handler, err := c.Request.FormFile("file")
		var filePath string
		var fileName string
		var downloadPath string
		if err != nil {
			log.Println("Get Post FormFile error: ", err)
		} else if err == nil {
			defer file.Close()

			fileName = handler.Filename
			filePath = filepath.Join(uploadPath, fileName)
			log.Println("saved filePath: ", filePath)
			out, err := os.Create(filePath)
			if err != nil {
				c.String(http.StatusInternalServerError, err.Error())
				return
			}
			defer out.Close()

			_, err = io.Copy(out, file)
			if err != nil {
				c.String(http.StatusInternalServerError, err.Error())
				return
			}
		}

		//submitTime := time.Now()
		if fileName != "" {
			downloadPath = filepath.Join("download", fileName)
		}
		err = InsertSubmission(textInput, downloadPath)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		c.Redirect(http.StatusSeeOther, "/")
	})

	//r.GET("/query", func(c *gin.Context) {
	r.GET("/", func(c *gin.Context) {
		submissions, err := QuerySubmissions()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query submissions"})
			return
		}
		log.Println("rows: ", submissions)
		c.HTML(http.StatusOK, "template.html", gin.H{
			"rows": submissions,
		})
	})

	//Deal with download
	r.GET("/download/:filename", func(c *gin.Context) {
		filename := c.Param("filename")
		file := filepath.Join(uploadPath, filename)
		c.File(file)
	})

	r.Run(":8080")
}
