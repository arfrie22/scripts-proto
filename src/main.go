package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/joho/godotenv"
)

var validate = validator.New()

type ErrorResponse struct {
	FailedField string
	Tag         string
	Value       string
}

func ValidateStruct(s interface{}) []*ErrorResponse {
	var errors []*ErrorResponse
	err := validate.Struct(s)
	if err != nil {
		for _, err := range err.(validator.ValidationErrors) {
			var element ErrorResponse
			element.FailedField = err.StructNamespace()
			element.Tag = err.Tag()
			element.Value = err.Param()
			errors = append(errors, &element)
		}
	}
	return errors
}

type ImageIndex struct {
	SchemaVersion int             `json:"schemaVersion"`
	MediaType     string          `json:"mediaType"`
	Manifests     []ImageManifest `json:"manifests"`
}

type ImageManifest struct {
	MediaType   string            `json:"mediaType"`
	Digest      string            `json:"digest"`
	Size        int               `json:"size"`
	Platform    Platform          `json:"platform"`
	Annotations map[string]string `json:"annotations,omitempty"` // Annotations are optional
}

type Platform struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
	Variant      string `json:"variant,omitempty"` // Variant is optional
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found")
	}

	app := fiber.New()
	app.Use(cors.New())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("scripts.mkr.cx")
	})

	app.Get("/validate", func(c *fiber.Ctx) error {
		var request struct {
			Container string `query:"container" validate:"required"`
		}

		arch := []string{
			"linux/amd64",
			"linux/arm64",
		}

		if err := c.QueryParser(&request); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		{
			errors := ValidateStruct(request)
			if errors != nil {
				return c.Status(fiber.StatusBadRequest).JSON(errors)
			}
		}

		ref, err := name.ParseReference(request.Container)
		if err != nil {
			return c.SendStatus(fiber.StatusBadRequest)
		}

		descriptor, err := remote.Get(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
		if err != nil {
			return c.SendStatus(fiber.StatusBadRequest)
		}

		var index ImageIndex
		err = json.Unmarshal(descriptor.Manifest, &index)
		if err != nil {
			return c.SendStatus(fiber.StatusBadRequest)
		}

		foundArchs := make(map[string]bool) // Use a map to track found architectures

		for _, manifest := range index.Manifests {
			platformStr := manifest.Platform.OS + "/" + manifest.Platform.Architecture
			fmt.Println(platformStr) // Print for demonstration

			foundArchs[platformStr] = true // Mark the architecture as found
		}

		// Check if all required architectures were found
		allFound := true
		for _, a := range arch {
			if !foundArchs[a] {
				allFound = false
				break
			}
		}

		if allFound {
			return c.SendStatus(fiber.StatusOK)
		} else {
			return c.Status(fiber.StatusBadRequest).SendString("Missing required platforms")
		}
	})

	app.Listen(":3000")
}
