package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
)

// port to listen on
var port = "8080"

// ApiGetFile is the structure of the json getFile object
type ApiGetFile struct {
	FileName string `json:"fileName"`
}

// ApiHashFiles is the structure of the json hash object
type ApiHashFiles struct {
	Files []string `json:"files"`
}

type returnObject struct {
	value string
	error string
}

var GitRef string
var Branch string

// handleError return the error
func handleError(error string) returnObject {
	return returnObject{
		value: "",
		error: error,
	}
}

// sendError send the error to the client
func sendError(c *gin.Context, err string) {
	c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err})
}

// getFileContent return the content of the file
func getFileContent(fileName string) returnObject {
	url := parseUrl(GitRef, Branch, fileName)
	if url.error != "" {
		return handleError(url.error)
	}

	resp := request(url.value)

	if resp.error == "The website does not exist" {
		return handleError("The file " + fileName + " does not exist")
	}

	return resp
}

// initialize the global variables
func initialize() returnObject {
	apiDetails := request("http://init_api:8081/details/")
	if apiDetails.error != "" {
		return handleError(apiDetails.error)
	}

	// convert the json string to a map
	result, err := stringToJson(apiDetails.value)
	if err != "" {
		return handleError(err)
	}

	GitRef = result["gitRef"]
	Branch = result["branch"]
	return returnObject{}
}

func stringToJson(jsonString string) (map[string]string, string) {
	// Declared an empty map interface
	var result map[string]string

	// Unmarshal or Decode the JSON to the interface.
	err := json.Unmarshal([]byte(jsonString), &result)
	if err != nil {
		return map[string]string{}, "The data is invalid"
	}

	return result, ""
}

// parseUrl parse the arguments and return the url to the row file
func parseUrl(girRef string, branch string, fileName string) returnObject {
	if girRef == "" || branch == "" {
		return handleError("The gitRef or branch is not initialized")
	}

	details := strings.Split(girRef, "/")
	if len(details) < 5 {
		return handleError("The github ref is invalid")
	}

	repoName := details[3]
	branchName := details[4][:len(details[4])-4]
	url := "https://raw.githubusercontent.com/" + repoName + "/" + branchName + "/" + branch + "/" + fileName

	return returnObject{value: url}
}

// return the request from a web page
func request(url string) returnObject {
	// get text content from web page with request
	resp, err := http.Get(url)
	if err != nil {
		return handleError("The request failed")
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	// read the response body
	body, err := ioutil.ReadAll(resp.Body)

	if string(body) == "404: Not Found" {
		return handleError("The website does not exist")
	}

	if err != nil {
		return handleError(err.Error())
	}

	return returnObject{value: string(body)}
}

// hash text by using the sha256 algorithm
func hash(text string) string {
	shaObject := sha256.New()
	shaObject.Write([]byte(text))
	return fmt.Sprintf("%x", shaObject.Sum(nil))
}

var getFileContentError string

// hash list of files
func hashFiles(listOfFiles []string) returnObject {
	getFileContentError = "" // reset the error
	arrayOfHashes := make([]string, len(listOfFiles))

	var wg sync.WaitGroup
	for i, file := range listOfFiles {
		wg.Add(1)
		go hashFile(file, i, arrayOfHashes, &wg)
	}
	wg.Wait()
	if getFileContentError != "" {
		return handleError(getFileContentError)
	}

	sumHush := ""
	for _, hash := range arrayOfHashes {
		sumHush += hash
	}
	return returnObject{value: hash(sumHush)}
}

func hashFile(file string, index int, arrayOfHashes []string, wg *sync.WaitGroup) {
	defer func() {
		wg.Done()
	}()

	if getFileContentError != "" {
		return
	}

	content := getFileContent(file)
	if content.error != "" {
		getFileContentError = content.error
		return
	}

	arrayOfHashes[index] = hash(content.value)
}

// api to get file content from GitHub directory
func getFileApi(c *gin.Context) {
	initialize()

	var details ApiGetFile

	if c.BindJSON(&details) != nil {
		sendError(c, "bad request")
		return
	}

	fileContent := getFileContent(details.FileName)
	if fileContent.error != "" {
		sendError(c, fileContent.error)
		return
	}
	c.IndentedJSON(http.StatusOK, gin.H{"fileContent": fileContent.value})
}

// api to hash the content of the given list of files
func hashFilesApi(c *gin.Context) {
	initialize()

	var details ApiHashFiles

	if c.BindJSON(&details) != nil {
		sendError(c, "bad request")
		return
	}
	if len(details.Files) == 0 {
		sendError(c, "no files to hash")
		return
	}

	sha := hashFiles(details.Files)
	if sha.error != "" {
		sendError(c, sha.error)
		return
	}

	c.IndentedJSON(http.StatusOK, gin.H{"hash": sha.value})
}

// the main function start the api
func main() {
	router := gin.Default()

	// create api
	router.POST("/get_file_content", getFileApi)
	router.POST("/hash_files", hashFilesApi)

	// run the server
	err := router.Run(":" + port)
	if err != nil {
		panic(err)
	}

}
