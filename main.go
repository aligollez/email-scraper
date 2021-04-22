package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/gocolly/colly"
)

var allowedDomain = []string{"www.prajwalkoirala.com"}

var emailsAdded map[string]int = make(map[string]int)

const parallelThreads = 25

//Infos info
type Infos struct {
	data map[string][]string
}

func writeToFile(data map[string][]string) {
	file, _ := json.MarshalIndent(data, "", " ")
	_ = ioutil.WriteFile("emails.json", file, 0644)
}

func main() {
	infos := Infos{data: make(map[string][]string)}
	// Instantiate default collector
	c := colly.NewCollector(
		colly.AllowedDomains(allowedDomain...),
	)
	c.AllowURLRevisit = false
	c.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: parallelThreads})
	c.OnHTML("html", func(e *colly.HTMLElement) {
		re := regexp.MustCompile(`[A-Za-z0-9.-]+@[A-Za-z0-9.-]+\.[A-Za-z0-9]{2,4}`)
		emails := re.FindAll([]byte(e.Text), -1)
		// fmt.Printf("%q\n", emails)
		for _, s := range emails {
			email := string(s)
			// fmt.Println(email)
			_, ok := emailsAdded[email]
			if !ok {
				emailsAdded[email] = 1
				infos.data[e.Request.URL.Host] = append(infos.data[e.Request.URL.Host], email)
			}
		}
		re2 := regexp.MustCompile(`[A-Za-z0-9]+\.[A-Za-z0-9]+\[at\][A-Za-z0-9]+\[dot\][A-Za-z0-9]+`)
		emails2 := re2.FindAll([]byte(e.Text), -1)
		for _, s := range emails2 {
			email := string(s)
			_, ok := emailsAdded[email]
			if !ok {
				// fmt.Println(email)
				infos.data[e.Request.URL.Host] = append(infos.data[e.Request.URL.Host], email)
			}
		}
	})
	// On every a element which has href attribute call callback
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		// Visit link found on page, Only those links are visited which are in AllowedDomains
		c.Visit(e.Request.AbsoluteURL(link))
	})
	// Before making a request print "Visiting ..."
	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL.String())
		if strings.Contains(r.URL.String(), ".css") || strings.Contains(r.URL.String(), ".js") || strings.Contains(r.URL.String(), ".jpg") || strings.Contains(r.URL.String(), ".png") || strings.Contains(r.URL.String(), ".gif") || strings.Contains(r.URL.String(), ".webp") || strings.Contains(r.URL.String(), ".psd") || strings.Contains(r.URL.String(), ".bmp") || strings.Contains(r.URL.String(), ".heif") || strings.Contains(r.URL.String(), ".indd") || strings.Contains(r.URL.String(), ".svg") || strings.Contains(r.URL.String(), ".ai") || strings.Contains(r.URL.String(), ".eps") || strings.Contains(r.URL.String(), ".pdf") {
			fmt.Println("Ignoring", r.URL.String())
			r.Abort()
		}
	})
	c.Visit("https://" + allowedDomain[0])
	c.Wait()
	writeToFile(infos.data)
}
