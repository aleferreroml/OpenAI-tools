package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/go-playground/validator/v10"
)

const (
	INPUT_PROMPT      = "Lionel Messi wearing a Boca Juniors jersey"
	NUMBER_OF_RESULTS = 3
	SIZE              = "512x512"
	RESPONSE_FORMAT   = "b64_json"
	OUTPUT_DIR        = "./output/"
)

type RequestBody struct {
	Prompt          string `json:"prompt" validate:"required,max=1000"`
	NumberOfResults int    `json:"n" validate:"omitempty,min=1,max=10"`
	Size            string `json:"size" validate:"omitempty,oneof=256x256 512x512 1024x1024"`
	ResponseFormat  string `json:"response_format" validate:"omitempty,oneof=url b64_json"`
}

type ResponseBody struct {
	Created int                      `json:"created"`
	Data    []map[string]interface{} `json:"data"`
}

type ResponseError struct {
	ApiError `json:"error"`
}

type ApiError struct {
	//	Code string
	Message string `json:"message"`
	Type    string `json:"type"`
}

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	client := &http.Client{}
	validate := validator.New()
	reqBody := RequestBody{
		Prompt:          INPUT_PROMPT,
		NumberOfResults: NUMBER_OF_RESULTS,
		Size:            SIZE,
		ResponseFormat:  RESPONSE_FORMAT,
	}

	err := validate.Struct(reqBody)
	if err != nil {
		log.Fatal(err)
	}

	var buf bytes.Buffer
	err = json.NewEncoder(&buf).Encode(reqBody)
	if err != nil {
		log.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/images/generations", &buf)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	response, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	defer response.Body.Close()

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err)
	}

	if response.StatusCode > 299 {
		var responseErr ResponseError
		_ = json.Unmarshal(bodyBytes, &responseErr)
		log.Printf("Request failed with response code %d | Status %s | Body: %+v", response.StatusCode, response.Status, responseErr)
		return
	}

	var responseObject ResponseBody
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		log.Fatal(err)
	}

	for i, img := range responseObject.Data {
		filename := GenerateFilename(INPUT_PROMPT, i+1)
		path := OUTPUT_DIR + filename
		if RESPONSE_FORMAT == "b64_json" {
			err = DecodeBase64AndSaveImg(img["b64_json"].(string), path)
		}

		if err != nil {
			log.Printf("Error al decodificar imagen %s | %s", filename, err.Error())
		}
	}
}

func GenerateFilename(seed string, index int) string {
	filename := strings.ReplaceAll(strings.Trim(seed, " "), " ", "_")

	return fmt.Sprintf("%s_%d.png", filename, index)
}

func DecodeBase64AndSaveImg(b64Img, output string) error {
	dec, err := base64.StdEncoding.DecodeString(b64Img)
	if err != nil {
		return err
	}

	f, err := os.Create(output)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Write(dec); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}

	return nil
}
