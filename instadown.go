package main

import (
    "encoding/json"
    "flag"
    "fmt"
    "io"
    "log"
    "net/http"
    "os"
    "path/filepath"
    "strings"
    "sync"
)

const Likes = "/api.instagram.com/v1/users/self/media/liked"

var (
    endpoint = flag.String("endpoint", "https://foauth.org", "The foauth endpoint to use")
    output   = flag.String("out", "Instadown", "The directory to put things in")
    email    = flag.String("email", "", "Email to authenticate foauth with")
    password = flag.String("password", "", "Password to authenticate foauth with")
)

type PaginationInfo struct {
    NextMaxLikeId *string `json:"next_max_like_id"`
}

type Resolution struct {
    Url string `json:"url"`
}

type Images struct {
    StandardResolution Resolution `json:"standard_resolution"`
}

type Caption struct {
    Text string `json:"text"`
}

type Like struct {
    Tags   []string `json:"tags"`
    Images Images   `json:"images"`
}

type Response struct {
    Pagination PaginationInfo `json:"pagination"`
    Likes      []Like         `json:"data"`
}

func downloadFile(url string) {
    parts := strings.Split(url, "/")
    filename, err := filepath.Abs(filepath.Join(*output, parts[len(parts)-1]))
    if err != nil {
        // This should never happen
        panic(err)
    }

    file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
    if err != nil {
        return
    }
    defer file.Close()

    resp, err := http.Get(url)
    if err != nil {
        return
    }
    defer resp.Body.Close()

    io.Copy(file, resp.Body)
}

func downloader(urls chan string, wg *sync.WaitGroup) {
    wg.Add(1)
    defer wg.Done()
    for url := range urls {
        downloadFile(url)
    }
}

func getLikes(nextMaxLikeId string) (*Response, error) {
    url := fmt.Sprintf("%s%s?max_like_id=%s", *endpoint, Likes, nextMaxLikeId)
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, err
    }
    req.SetBasicAuth(*email, *password)
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    decoder := json.NewDecoder(resp.Body)
    response := new(Response)
    err = decoder.Decode(response)
    return response, err
}

func downloadLikesRecursive(nextMaxLikeId string, urls chan string) {
    resp, err := getLikes(nextMaxLikeId)
    if err != nil {
        log.Printf("failed getting likes: %s", err)
        return
    }

    for _, like := range resp.Likes {
        urls <- like.Images.StandardResolution.Url
    }

    if resp.Pagination.NextMaxLikeId != nil {
        downloadLikesRecursive(*resp.Pagination.NextMaxLikeId, urls)
    }
}

func downloadLikes() {
    log.Printf("downloading likes to %#v", *output)

    var wg sync.WaitGroup
    urls := make(chan string, 50)

    go downloader(urls, &wg)
    go downloader(urls, &wg)
    go downloader(urls, &wg)

    downloadLikesRecursive("", urls)
    close(urls)

    wg.Wait()
}

func main() {
    flag.Parse()
    downloadLikes()
}
