package main

// the packages that we need.
import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"runtime"
)

// the location of documents.
const (
	persistentFileName = "persistent.txt"
	inputFileName      = "input.json"
	outputFileName     = "output.json"
	domainsFileName    = "domains.txt"
)

// Using this regex to locate emails that appear on a document file
var (
	findCommonRegexp = regexp.MustCompile(`[A-Za-z0-9.-]+@[A-Za-z0-9.-]+\.[A-Za-z0-9]{2,4}`)
)

func clearCache() {
	switch runtime.GOOS {
	case "windows", "darwin", "linux":
		os.RemoveAll(os.TempDir())
	default:
		fmt.Println("Error: Temporary files can't be deleted.")
	}
}

// Emails contained in the file are checked
func validateEmail(domains map[string]struct{}, email string) bool {
	emailResp1 := domainCheck(domains, email)
	if !emailResp1 {
		return false
	}
	emailResp2 := syntaxCheck(email)
	if !emailResp2 {
		return false
	}
	return true
}

// Don't read the same email tiwce; go to the next one.
func Find(haystack []byte) (emails []string) {
	results := findCommonRegexp.FindAll(haystack, -1)
	for _, r := range results {
		emails = append(emails, string(r))
	}
	return emails
}

type EmailField struct {
	Email string `json:"Email"`
}

// Perform a test of the syntax on the found documents.
func syntaxCheck(email string) bool {
	re := regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
	return re.MatchString(email)
}

// Checking and validating the e-mail domain; validating the domain name.
func domainCheck(domains map[string]struct{}, email string) bool {
	if !strings.Contains(email, "@") {
		return false
	}

	domain := strings.Split(email, "@")[1]
	if checkDomainInFile(domains, domain) {
		return true
	}

	nameservers, _ := net.LookupNS(domain)
	if len(nameservers) > 0 {
		if err := saveDomainInFile(domain); err != nil {
			log.Panic(err)
		}

		domains[domain] = struct{}{}
		return true
	}

	return false
}

func main() {
	clearCache()
	emails := map[string]struct{}{}
	domains := map[string]struct{}{}

	readEmails(emails)
	readDomains(domains)

	// We use persistent and store the bytes read so if the program crashes; we don't have to read the file from the start.
	persistentFile, _ := os.Open(persistentFileName)

	persistReader := bufio.NewReader(persistentFile)
	persistBytes, _, _ := persistReader.ReadLine()
	curNum, _ := strconv.Atoi(string(persistBytes))

	_ = persistentFile.Close()

	inputFile, _ := os.Open(inputFileName)
	inputReader := bufio.NewReader(inputFile)
	_, _ = inputReader.Discard(curNum)

	for {
		line, _, err := inputReader.ReadLine()
		if err != nil {
			if err != io.EOF {
				log.Fatal(err)
			}
			break
		}

		curNum += len(string(line)) + 1
		emailsInLine := Find(line)

		for _, e := range emailsInLine {
			oneDoc := EmailField{
				Email: e,
			}

			if !isUnique(emails, e) {
				continue
			}

			if !validateEmail(domains, e) {
				continue
			}

			//fmt.Println("Email:", e)

			err = saveEmailInFile(oneDoc)
			if err != nil {
				log.Println("internal save file err", err)
				continue
			}
			emails[e] = struct{}{}
		}

		// wf.Truncate(0)
		// wf.Seek(0, 0)
		wf, err := os.Create(persistentFileName)
		writer := bufio.NewWriter(wf)

		if err != nil {
			log.Fatal(err)
		}

		_, err = writer.WriteString(strconv.Itoa(curNum))
		if err != nil {
			fmt.Println(err)
		}

		_ = writer.Flush()
		_ = wf.Close()
	}

	_ = inputFile.Close()
	fmt.Println()
}

// Don't save an email twice.
func isUnique(emails map[string]struct{}, email string) bool {
	_, ok := emails[email]
	return !ok
}

// Checks if a domain is located on the document.
func checkDomainInFile(domains map[string]struct{}, domain string) bool {
	_, ok := domains[domain]
	return ok
}

// Save valid domains on a text file and do not need to do another NS lookup on the same domain.
func saveDomainInFile(domain string) error {
	file := OpenExistingFileOrCreate(domainsFileName, "write")
	_, _ = fmt.Fprintln(file, domain)
	return file.Close()
}

// saves the valid emails on the file
func saveEmailInFile(field EmailField) error {
	file := OpenExistingFileOrCreate(outputFileName, "write")
	encoder := json.NewEncoder(file)
	_ = encoder.Encode(field)
	return file.Close()
}

// Creates a file if it is not found; uses the file if found.
func OpenExistingFileOrCreate(fileName string, mode string) *os.File {
	var file *os.File
	var err error
	if mode == "write" {
		file, err = os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	} else {
		file, err = os.Open(fileName)
	}

	if err != nil {
		file, err = os.Create(fileName)
		if err != nil {
			panic(err)
		}
	}
	return file
}

// Read the emails on file; and than decide what to do.
func readEmails(emails map[string]struct{}) {
	file := OpenExistingFileOrCreate(outputFileName, "")
	persistReader := bufio.NewReader(file)
	for {
		line, _, err := persistReader.ReadLine()
		if err != nil {
			if err != io.EOF {
				log.Fatal(err)
			}
			return
		}

		email := EmailField{}
		_ = json.Unmarshal(line, &email)
		emails[email.Email] = struct{}{}
	}
}

// Read the domains on the file and decide what to do.
func readDomains(emails map[string]struct{}) {
	file := OpenExistingFileOrCreate(domainsFileName, "")
	persistReader := bufio.NewReader(file)
	for {
		line, _, err := persistReader.ReadLine()
		if err != nil {
			if err != io.EOF {
				log.Fatal(err)
			}
			return
		}
		emails[string(line)] = struct{}{}
	}
}
