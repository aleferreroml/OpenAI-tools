package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-playground/validator/v10"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	INPUT_PROMPT      = "write the name 'John' and the number 9 on the jersey as a soccer jersey using Times New Roman Font."
	NUMBER_OF_RESULTS = 3
	SIZE              = "512x512"
	RESPONSE_FORMAT   = "b64_json"
	OUTPUT_DIR        = "./output/"
	IMAGE             = "./images/boca_blanca.png"
	MASK              = "./images/mask_boca_blanca.png"
)

type RequestBody struct {
	Image           *os.File `validate:"required"`
	Mask            *os.File `validate:"omitempty"`
	Prompt          string   `validate:"required,max=1000"`
	NumberOfResults int      `validate:"omitempty,min=1,max=10"`
	Size            string   `validate:"omitempty,oneof=256x256 512x512 1024x1024"`
	ResponseFormat  string   `validate:"omitempty,oneof=url b64_json"`
}

type ResponseBody struct {
	Created int                      `json:"created"`
	Data    []map[string]interface{} `json:"data"`
}

type ResponseError struct {
	ApiError `json:"error"`
}

type ApiError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	client := &http.Client{}
	validate := validator.New()
	reqBody := RequestBody{
		Image:           mustOpen(IMAGE),
		Mask:            mustOpen(MASK),
		Prompt:          INPUT_PROMPT,
		NumberOfResults: NUMBER_OF_RESULTS,
		Size:            SIZE,
		ResponseFormat:  RESPONSE_FORMAT,
	}

	err := validate.Struct(reqBody)
	if err != nil {
		log.Fatal(err)
	}

	err = ValidateImage(reqBody.Image)
	if err != nil {
		log.Fatal(err)
	}

	if reqBody.Mask != nil {
		err = ValidateImage(reqBody.Mask)
		if err != nil {
			log.Fatal(err)
		}
	}

	body := ParseStructRequestBody(reqBody)

	var buf bytes.Buffer
	contentType, err := CreateFormData(&buf, body)
	if err != nil {
		log.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/images/edits", &buf)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Content-Type", contentType)
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

func ParseStructRequestBody(reqBody RequestBody) map[string]io.Reader {
	return map[string]io.Reader{
		"image":           reqBody.Image,
		"mask":            reqBody.Mask,
		"prompt":          strings.NewReader(reqBody.Prompt),
		"n":               strings.NewReader(fmt.Sprintf("%d", reqBody.NumberOfResults)),
		"size":            strings.NewReader(reqBody.Size),
		"response_format": strings.NewReader(reqBody.ResponseFormat),
	}
}

// CreateFormData Prepare a form that you will submit to that URL.
func CreateFormData(buf *bytes.Buffer, values map[string]io.Reader) (string, error) {
	mpWriter := multipart.NewWriter(buf)

	for key, r := range values {
		var fw io.Writer
		var err error
		if x, ok := r.(io.Closer); ok {
			defer x.Close()
		}
		// Add an image file
		if x, ok := r.(*os.File); ok {
			if fw, err = mpWriter.CreateFormFile(key, x.Name()); err != nil {
				return "", err
			}
		} else {
			// Add other fields
			if fw, err = mpWriter.CreateFormField(key); err != nil {
				return "", err
			}
		}
		if _, err := io.Copy(fw, r); err != nil {
			return "", err
		}
	}
	mpWriter.Close()

	return mpWriter.FormDataContentType(), nil
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

func mustOpen(filename string) *os.File {
	r, err := os.Open(filename)
	if err != nil {
		panic(err)
	}
	return r
}

// ValidateImage valida que la imagen sea un archivo PNG menor a 4MB.
func ValidateImage(img *os.File) error {
	fileInfo, err := img.Stat()
	if err != nil {
		return err
	}
	if filepath.Ext(fileInfo.Name()) != ".png" {
		return errors.New("The image extension must be PNG")
	}
	if fileInfo.Size() > 4000000 { //4MB
		return errors.New("The size of the image must not exceed 4MB")
	}
	//TODO falta chequear si la imagen es cuadrada y si la mask tiene las mismas dimensiones que la imagen original

	return nil
}
