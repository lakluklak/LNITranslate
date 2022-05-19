package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"
)

const root = "."
const rootNew = "translated\\"

var runProcesses int
var wg *sync.WaitGroup

func scanString(inp string) string { //перевод, если текст русский
	inp = strings.Replace(inp, "\\\"", "''", -1) // замена ковычек, чтобы корректно переводилось
	for _, char := range []rune(inp) {
		if unicode.Is(unicode.Cyrillic, char) { // если есть русский символ - переводим
			return translate(inp)
		}
	}
	return inp
}

func translate(inp string) string { //перевод текста
	resp, err := http.Get("http://translate.googleapis.com/translate_a/single?client=gtx&sl=ru&tl=en&dt=t&q=" + url.QueryEscape(inp))
	if err != nil {
		panic("Translate api error")
	}
	body, _ := io.ReadAll(resp.Body)
	var objmap [][][]interface{}
	json.Unmarshal(body, &objmap)
	result := ""
	for _, elem := range objmap[0] {
		result += elem[0].(string)
	}
	return result
}

func checkBackSlash(text []rune, startindex, index int) bool { //проверка на экранирование ковычек
	slashCount := 0
	for ; index > startindex; index-- {
		if slashCount == 0 && text[index-1] != '\\' {
			return true
		} else if slashCount == 0 && text[index-1] == '\\' {
			slashCount++
		} else if slashCount == 1 && text[index-1] == '\\' {
			slashCount = 0
		} else if slashCount == 1 && text[index-1] != '\\' {
			return false
		}
	}
	return false
}

func removeNoise(path string) {
	result := "" // то, что мы запишем в переведенный файл
	temp := ""   // символы разметки
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	s := bufio.NewScanner(file)
	stringBuf := "" // текст для перевода
	lineN := 0      // текущая строка для вывода в консоль информации о ходе перевода
	for s.Scan() {
		lineN++
		if lineN%1000 == 0 {
			fmt.Println("file:", path, "line:", lineN)
		}
		startIndex := 0 //до какой позиции проверять ковычки на экранирование
		str := []rune(s.Text())
		skip := 0
		skipLine := false
		for i := range str {
			if skip > 0 { // пропуск skip символов
				result += string(str[i])
				skip--
				continue
			}
			if skipLine { // пропуск строки
				break
			}
			switch temp {
			case "": // если не встретили символ разметки
				if str[i] == '"' {
					if str[i+1] == '"' {
						skip += 1
					} else {
						temp = "\""
						startIndex = i + 1
					}
				} else if len(str) >= i+3 {
					if str[i] == '[' && str[i+1] == '=' && str[i+2] == '[' {
						temp = "[=["
						skip += 2
					}
				}
				if unicode.Is(unicode.Cyrillic, str[i]) || strings.Contains(path, ".txt") {
					result += scanString(string(str[i:])) // прибавляем переведенный текст
					skipLine = true
				} else {
					result += string(str[i])
				}
			case "\"": // встретили символ разметки ", ищем закрывающую ковычку
				if str[i] == '"' && checkBackSlash(str, startIndex, i) { // checkBackSlash - функция для проверки экранирования ковычек
					result += scanString(stringBuf) + string(str[i]) // прибавляем переведенный текст
					temp = ""
					stringBuf = ""
				} else {
					stringBuf += string(str[i])
				}
			case "[=[": // встретили символ разметки [=[, ищем закрывающие скобки
				if len(str) >= i+3 {
					if str[i] == ']' && str[i+1] == '=' && str[i+2] == ']' {
						result += scanString(stringBuf) + string(str[i]) // прибавляем переведенный текст
						temp = ""
						stringBuf = ""
					} else {
						stringBuf += string(str[i])
					}
				} else {
					stringBuf += string(str[i])
				}
			}
		}
		if len(stringBuf) > 0 {
			stringBuf += "\n"
		} else {
			result += "\n"
		}
	}
	file.Close()

	// запись результата в файл
	file, err = os.Create(rootNew + path)
	if err != nil {
		panic(err)
	}
	w := bufio.NewWriter(file)
	w.WriteString(result)
	w.Flush()
	file.Close()
	fmt.Println(path, "Done!")
	wg.Done()
}

func walkFunc(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	if info.IsDir() {
		if strings.Contains(rootNew, path) || info.Name() == ".idea" { //Проверяем является ли папка сгенерированной Goland или хранящей переведенные файоы
			return filepath.SkipDir // является - пропускаем
		} else {
			os.MkdirAll(rootNew+path, os.ModePerm) // не является - создаём в папке для переведённых файлов дирректорию с таким же путем
		}
		return nil
	}
	if strings.Contains(info.Name(), ".ini") || strings.Contains(info.Name(), ".txt") || strings.Contains(info.Name(), ".j") {
		//проверяем, имеют ли файлы нужное расширение
		wg.Add(1)
		go removeNoise(path) // запускаем горутину для перевода

	}

	return nil
}
func run() {
	wg = new(sync.WaitGroup)                              //переменная для синхронизации
	os.MkdirAll(rootNew, os.ModePerm)                     //создаём папку для переведенных файлов
	if err := filepath.Walk(root, walkFunc); err != nil { //обходим все папки и файлы в корневой папке
		panic("run, Get error: " + err.Error())
	}
	wg.Wait() //ожидаем завершения всех горутин
}
func main() {
	run()
}
