package main

import (
	"database/sql"
	"flag"
	"fmt"
	"image"
	_ "image/png"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v2"
)

//T is the spec of site.yml
type T struct {
	//host in the format like www.example.com
	host string
	//title keyword1-keyword2-keyword3-keyword4
	title string
	//meta keyword: keyword1,keyword2,keyword3,keyword4
	keywords string
	//meta descriptions: its length should be greather than 50 and less than 150
	descriptions string
	//baidu track ID
	baiduTrackID string
}

var logger *zap.Logger
var pool *sql.DB // Database connection pool.
var dbName string
var config T

func getMatch(content, pattern string) string {
	r, _ := regexp.Compile(pattern)
	matches := r.FindStringSubmatch(content)
	return matches[1]
}
func readFileContent(fileName string) string {
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		logger.Fatal("file not exists", zap.String("filename", fileName))
	}
	content, err := ioutil.ReadFile(fileName)
	if err != nil {
		logger.Fatal("fail to read file ", zap.String("filename", fileName))
	}
	return string(content)
}
func checkDB(dbFile string) {
	text := readFileContent(dbFile)
	dbHost := getMatch(text, `\$cfg_dbhost = '(.*)'`)
	dbName = getMatch(text, `\$cfg_dbname = '(.*)'`)
	dbUser := getMatch(text, `\$cfg_dbuser = '(.*)'`)
	dbPwd := getMatch(text, `\$cfg_dbpwd = '(.*)'`)
	logger.Info("db info",
		zap.String("dbhost", dbHost), zap.String("dbname", dbName),
		zap.String("dbuser", dbUser),
		zap.String("dbpwd", dbPwd))
	connStr := fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?charset=utf8&parseTime=True&loc=Local",
		dbUser, dbPwd, dbHost, dbName)
	//logger.Info("connetion string", zap.String("connection_string", connStr))
	var err error
	pool, err = sql.Open("mysql", connStr)
	if err != nil {
		panic(err.Error())
	}
	defer pool.Close()
	checkTypeDir()
	checkSysConf()
	//check cfg_basehost  should be www.
	//cfg_arcdir:  should be /
	//cfg_keywords
	//cfg_description
	checkMismatchDK()
}
func checkMismatchDK() {
	stmt := fmt.Sprintf(`select * 
from (select typename, 
(select count(*) from %s.dede_sysconfig ds
   where ds.varname = 'cfg_keywords'
   and ds.value = da.keywords ) keywordMatch, 
   (select count(*) from %s.dede_sysconfig ds
   where ds.varname = 'cfg_description'
   and ds.value = da.description ) descMatch
from   %s.dede_arctype da
) a where keywordMatch = 0 or descMatch = 0`, dbName, dbName, dbName)
	rows, err := pool.Query(stmt)
	if err != nil {
		// handle this error better than this
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		var typename string
		var keyMatch int
		var descMatch int
		err = rows.Scan(&typename, &keyMatch, &descMatch)
		if err != nil {
			// handle this error
			panic(err)
		}
		if keyMatch == 0 {
			logger.Error("keyworkd of site and column mismatch", zap.String("column_name", typename))
		}
		if descMatch == 0 {
			logger.Error("descriptions of site and column mismatch", zap.String("column_name", typename))
		}
		// get any error encountered during iteration
		err = rows.Err()
		if err != nil {
			panic(err)
		}
	}
}
func checkSysConf() {

	stmt := fmt.Sprintf(`select varname, value
from %s.dede_sysconfig ds 
where ( varname  = 'cfg_basehost'  and instr(value,?) = 0 )
or (varname = 'cfg_arcdir' and value like '/a')
or (varname = 'cfg_keywords' and value <> ? )
or (varname = 'cfg_description' and value <> ? ) 
or (varname ='cfg_webname' and value <> ? )`, dbName)

	rows, err := pool.Query(stmt, config.host, config.keywords, config.descriptions, config.title)
	if err != nil {
		// handle this error better than this
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		var varname string
		var value string
		err = rows.Scan(&varname, &value)
		if err != nil {
			// handle this error
			panic(err)
		}
		logger.Error("系统配置参数", zap.String("varname", varname),
			zap.String("value", value))
		// get any error encountered during iteration
		err = rows.Err()
		if err != nil {
			panic(err)
		}
	}
}
func checkTypeDir() {

	stmt := fmt.Sprintf(`select typename , typedir from %s.dede_arctype da where typedir like '%%/a/%%'`, dbName)
	//logger.Info(sql)
	rows, err := pool.Query(stmt)
	if err != nil {
		// handle this error better than this
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		var typename string
		var typedir string
		err = rows.Scan(&typename, &typedir)
		if err != nil {
			// handle this error
			panic(err)
		}
		logger.Error("archive save dir in {cmspath}/a", zap.String("栏目名称", typename),
			zap.String("typedir", typedir))
		// get any error encountered during iteration
		err = rows.Err()
		if err != nil {
			panic(err)
		}
	}
}
func checkTemplateCommon(fileName, content string) {
	//logger.Info("check og:imgage, bdsjs, skin", zap.String("file_name", fileName))
	//og:image
	ogImg := `<meta property="og:image" content="/skin/images/logo.png" />`
	checkTag(content, ogImg, fileName, "og:image")
	//bds.js
	bdsjs := `<script type='text/javascript' src='/skin/js/bds.js'></script>`
	checkTag(content, bdsjs, fileName, "bdsjs")
	//css
	//js
	re1 := regexp.MustCompile(`"([a-zAZ\/0-9._\-]*?\.js)|([a-zAZ\/0-9_\-.]*?\.css)"`)
	re2 := regexp.MustCompile(`/skin`)
	lines := strings.Split(content, `\n`)
	match := false
	for i, line := range lines {
		match = re1.MatchString(line)
		if match {
			match = re2.MatchString(line)
			if !match {
				logger.Error("js or css file is not in skin folder",
					zap.Int("line_no", i+1),
					zap.String("file_name", fileName))
			}
		}
	}
	//TODO: alt
	//count image
}
func checkTemplateFileIndex(fileName string) {
	title := `<title>{dede:global.cfg_webname/}</title>`
	keywords := `<meta name="keywords" content="{dede:global.cfg_keywords/}" />`
	descriptions := `<meta name="description" content="{dede:global.cfg_description/}" />`
	full := `<title>{dede:global.cfg_webname/}</title>
<meta name="keywords" content="{dede:global.cfg_keywords/}" />
<meta name="description" content="{dede:global.cfg_description/}" />
<meta property="og:image" content="/skin/images/logo.png" />
<script type='text/javascript' src='/skin/js/bds.js'></script>`
	checkTemplateFile(fileName, full, func(content string) {
		checkTag(content, title, fileName, "title")
		checkTag(content, keywords, fileName, "keywords")
		checkTag(content, descriptions, fileName, "descriptions")
	})
}
func checkTemplateFileList(fileName string) {

	title := `<title>{dede:field.typename/}_{dede:global.cfg_webname/}</title>`
	keywords := `<meta name="keywords" content="{dede:field name='keywords'/}" />`
	descriptions := `<meta name="description" content="{dede:field name='description' function='html2text(@me)'/}" />`
	full := `<title>{dede:field.typename/}_{dede:global.cfg_webname/}</title>
<meta name="keywords" content="{dede:field name='keywords'/}" />
<meta name="description" content="{dede:field name='description' function='html2text(@me)'/}" />
<meta property="og:image" content="/skin/images/logo.png" />
<script type='text/javascript' src='/skin/js/bds.js'></script>`
	checkTemplateFile(fileName, full, func(content string) {
		checkTag(content, title, fileName, "title")
		checkTag(content, keywords, fileName, "keywords")
		checkTag(content, descriptions, fileName, "descriptions")
	})
}
func checkTemplateFileArticle(fileName string) {
	title := `<title>{dede:field.title/}_{dede:global.cfg_webname/}</title>`
	keywords := `<meta name="keywords" content="{dede:field name='keywords'/}" />`
	descriptions := `<meta name="description" content="{dede:field name='description' function='html2text(@me)'/}" />`
	full := `<title>{dede:field.title/}_{dede:global.cfg_webname/}</title>
<meta name="keywords" content="{dede:field name='keywords'/}" />
<meta name="description" content="{dede:field name='description' function='html2text(@me)'/}" />
<meta property="og:image" content="/skin/images/logo.png" />
<script type='text/javascript' src='/skin/js/bds.js'></script>`
	checkTemplateFile(fileName, full, func(content string) {
		checkTag(content, title, fileName, "title")
		checkTag(content, keywords, fileName, "keywords")
		checkTag(content, descriptions, fileName, "descriptions")
	})
}
func checkTag(content, pattern, fileName, key string) {
	if !strings.Contains(content, pattern) {
		logger.Error(fmt.Sprintf("tag %s doesn't comform to the %s formats", key, key),
			zap.String("file_name", fileName),
			zap.String(fmt.Sprintf("%s_format", key), pattern))
	}
}
func checkTagMiscOrder(content, pattern, fileName string) {
	if !strings.Contains(content, pattern) {
		logger.Error("title, keywords, description, og:image and bds.js order is out of line with spec",
			zap.String("file_name", fileName),
			zap.String("tag_order", "title, keywords,description, og:image, bds.js"))
	}
}

func checkTemplateFile(fileName, full string, fnCheck func(str string)) {
	logger.Info("checking template file ... ", zap.String("file_name", fileName))
	content := readFileContent(fileName)
	//title
	//keyworkd
	//description
	fnCheck(content)
	//og:image
	//bds
	//css
	//js
	checkTemplateCommon(fileName, content)
	//order of above five items
	checkTagMiscOrder(content, full, fileName)
}
func main() {
	initLogger()
	defer logger.Sync()

	flag.Set("alsologtostderr", "true")
	flag.Parse()
	if len(os.Args) == 1 {
		logger.Info("a path command argument should be provided")
		return
	}
	path := os.Args[1]
	if _, err := os.Stat(path); os.IsNotExist(err) {
		logger.Fatal("path is not existed", zap.String("path", path))
	}
	dbFile := path + string(os.PathSeparator) + "data" + string(os.PathSeparator) + "common.inc.php"
	logger.Info("common.inc.php path", zap.String("path", dbFile))
	parseConfig(path)
	checkDB(dbFile)
	checkTemplateFileIndex(filepath.FromSlash(path + "/templets/default/index.htm"))
	checkTemplateFileList(filepath.FromSlash(path + "/templets/default/list_article.htm"))
	checkTemplateFileArticle(filepath.FromSlash(path + "/templets/default/article_article.htm"))
	fileExists(path)

	return
}
func getImageDimension(imagePath string) (int, int) {
	file, err := os.Open(imagePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
	}

	image, _, err := image.DecodeConfig(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", imagePath, err)
	}
	return image.Width, image.Height
}
func fileExists(path string) {
	logger.Info("checking image format, size, bds content ...")
	imagePath := filepath.FromSlash(path + "/skin/images/logo.png")
	_, err := os.Stat(imagePath)
	if os.IsNotExist(err) {
		logger.Error("/skin/images/logo.png is not exist")
	}
	if !os.IsNotExist(err) {
		w, h := getImageDimension(imagePath)
		if w != 120 || h != 75 {
			logger.Error("logo dimension is not 120x75",
				zap.String("file_name", imagePath),
				zap.Int("width", w),
				zap.Int("height", h))
		}
	}
	_, err = os.Stat(filepath.FromSlash(path + "/skin/images/defautpic.gif"))
	if os.IsNotExist(err) {
		logger.Error("/skin/images/defaultpic.gif is not exist")
	}
	bds := filepath.FromSlash(path + "/skin/js/bds.js")
	_, err = os.Stat(bds)
	if os.IsNotExist(err) {
		logger.Error("/skin/js/bds.js is not exist")
	}
	if !os.IsNotExist(err) {
		data, err := ioutil.ReadFile(bds)
		if err != nil {
			logger.Error("Failed to read bds.js", zap.String("bds", bds))
		}
		text := string(data)
		if !strings.Contains(text, config.baiduTrackID) {
			logger.Error("mismatch baidu Track ID", zap.String("file_name", bds))
		}
	}
}
func initLogger() {
	//logger, _ = zap.NewProduction()
	//config := zap.NewProductionConfig()
	config := zap.NewDevelopmentConfig()
	config.DisableStacktrace = true
	config.DisableCaller = true
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	logger, _ = config.Build()
}
func parseConfig(basePath string) {
	_, err := os.Stat(filepath.FromSlash(basePath + "/site.yml"))
	if !os.IsNotExist(err) {
		data, err := ioutil.ReadFile(filepath.FromSlash(basePath + "/site.yml"))
		if err != nil {
			err := yaml.Unmarshal([]byte(data), &config)
			if err != nil {
				log.Fatalf("error: %v", err)
			}
		}
	}
}
