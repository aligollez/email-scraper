package main

// the packages that we need.
import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

var persistentFileName string
var inputFileName string
var outputFileName string
var domainsFileName string
var findCommonRegexp string

var (
	emails = map[string]struct{}{}
	domains = map[string]struct{}{}
)

type EmailField struct {
	Email string `json:"Email"`
}

func init() {
	// User input flags
	if len(os.Args) > 1 {
		tempPersistentFileName := flag.String("persistent", "persistent.txt", "The ammout of bytes read")
		tempInputFileName := flag.String("input", "input.json", "The input file for email list")
		tempOutputFileName := flag.String("output", "output.json", "The output file for email list")
		tempDomainsFileName := flag.String("domains", "domains.txt", "Validated domains list")
		tempFindCommonRegexp := flag.String("regex", `[A-Za-z0-9.-]+@[A-Za-z0-9.-]+\.[A-Za-z0-9]{2,4}`, "Regex for emails")
		flag.Parse()
		persistentFileName = *tempPersistentFileName
		inputFileName = *tempInputFileName
		outputFileName = *tempOutputFileName
		domainsFileName = *tempDomainsFileName
		findCommonRegexp = *tempFindCommonRegexp
	} else {
		persistentFileName = "persistent.txt"
		inputFileName = "input.json"
		outputFileName = "output.json"
		domainsFileName = "domains.txt"
		findCommonRegexp = `[A-Za-z0-9.-]+@[A-Za-z0-9.-]+\.[A-Za-z0-9]{2,4}`
	}
	// Clear the temp files to get more space
	switch runtime.GOOS {
	case "windows", "darwin", "linux":
		os.RemoveAll(os.TempDir())
	default:
		log.Println("Error: Temporary files can't be deleted.")
	}
}

func main() {
	// Read all the emails on file
	readEmails(emails)
	// Read all the domains on file
	readDomains(domains)
	// We use persistent and store the bytes read so if the program crashes; we don't have to read the file from the start.
	persistentFile, _ := os.Open(persistentFileName)
	persistReader := bufio.NewReader(persistentFile)
	persistBytes, _, _ := persistReader.ReadLine()
	curNum, _ := strconv.Atoi(string(persistBytes))
	_ = persistentFile.Close()
	// Open the inputfile
	inputFile, _ := os.Open(inputFileName)
	// Read the input file
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
			fmt.Println("Email:", e)
			err = saveEmailInFile(oneDoc)
			if err != nil {
				log.Println("internal save file err", err)
				continue
			}
			emails[e] = struct{}{}
		}
		wf, err := os.Create(persistentFileName)
		if err != nil {
			log.Fatal(err)
		}
		writer := bufio.NewWriter(wf)
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
	regexValue := regexp.MustCompile(findCommonRegexp)
	results := regexValue.FindAll(haystack, -1)
	for _, r := range results {
		emails = append(emails, string(r))
	}
	return emails
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
