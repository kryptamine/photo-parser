package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"sync"

	"time"

	"strings"

	"os"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/vorkytaka/easyvk-go/easyvk"
)

const ImageSize = 807

var vk VkConnector

type (
	Image struct {
		Url    string `json:"url"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	}

	User struct {
		Id        int    `json:"id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Photo50   string `json:"photo_50"`
	}

	Item struct {
		Photo75  string `json:"photo_75"`
		Photo130 string `json:"photo_130"`
		Photo604 string `json:"photo_604"`
		Photo807 string `json:"photo_807"`
		Width    int    `json:"width"`
		Height   int    `json:"height"`
	}

	PhotoResponse struct {
		Items []Item `json:"items"`
	}

	VkConnector struct {
		Vk easyvk.VK
	}
)

func (pr PhotoResponse) collectItems(c chan []Image, group *sync.WaitGroup) {
	var images []Image

	for _, item := range pr.Items {
		image := Image{
			Url:    item.Photo807,
			Width:  0,
			Height: 0,
		}

		if item.Width > item.Height {
			if item.Width > 0 {
				image.Width = ImageSize
				image.Height = (ImageSize * item.Height) / item.Width
			}
		} else {
			if item.Height > 0 {
				image.Height = ImageSize
				image.Width = (ImageSize * item.Width) / item.Height
			}
		}

		if image.Url != "" {
			images = append(images, image)
		}
	}

	c <- images

	group.Done()
}

func shuffle(images []Image) []Image {
	for i := range images {
		j := rand.Intn(i + 1)
		images[i], images[j] = images[j], images[i]
	}

	return images
}

func (vk *VkConnector) connect(login string, password string, clientId string, scope string) error {
	vkConn, err := easyvk.WithAuth(login, password, clientId, scope)

	if err == nil {
		vk.Vk = vkConn
	}

	return err
}

func (vk *VkConnector) parse(id string) PhotoResponse {
	var photoResponse PhotoResponse

	params := map[string]string{
		"owner_id": id,
	}

	result, _ := vk.Vk.Request("photos.getAll", params)

	json.Unmarshal(result, &photoResponse)

	return photoResponse
}

func GetPhotos(w http.ResponseWriter, r *http.Request) {

	ids := r.URL.Query()["id[]"]

	if len(ids) == 0 {
		http.Error(w, "User IDs must be set", 400)

		return
	}

	var wg sync.WaitGroup

	wg.Add(len(ids))

	done := make(chan []Image)

	for _, id := range ids {
		go vk.parse(id).collectItems(done, &wg)
	}

	var result []Image

	go func(result *[]Image) {
		for response := range done {
			*result = append(*result, response...)
		}

	}(&result)

	wg.Wait()

	time.Sleep(time.Second * 1)

	json.NewEncoder(w).Encode(shuffle(result))
}

func GetUser(w http.ResponseWriter, r *http.Request) {
	ids := r.URL.Query()["id[]"]

	if len(ids) == 0 {
		http.Error(w, "User IDs must be set", 400)

		return
	}

	params := map[string]string{
		"user_ids": strings.Join(ids, ","),
		"fields":   "photo_50",
	}

	var users []User

	result, err := vk.Vk.Request("users.get", params)

	err = json.Unmarshal(result, &users)

	if err != nil {
		http.Error(w, err.Error(), 500)

		return
	}

	json.NewEncoder(w).Encode(users)
}

func main() {
	router := mux.NewRouter()

	err := godotenv.Load()

	if err != nil {
		log.Fatal("Error loading .env file")
	}

	if err := vk.connect(os.Getenv("VK_LOGIN"), os.Getenv("VK_PASSWORD"), os.Getenv("VK_CLIENT_ID"), os.Getenv("VK_SCOPE")); err != nil {
		log.Fatal(err)
	}

	router.HandleFunc("/photos", GetPhotos).Methods("GET")
	router.HandleFunc("/user", GetUser).Methods("GET")

	log.Fatal(http.ListenAndServe(":8000", handlers.CORS()(router)))
}
