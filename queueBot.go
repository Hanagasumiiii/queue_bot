package main

import (
	"encoding/json"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"log"
	"os"
	"sync"
)

var numericKeyboard = tgbotapi.NewReplyKeyboard(
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("Удалить очередь"),
	),
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("Новая очередь"),
	),
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("Показать списки"),
	),
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("Войти в очередь"),
	),
)

var userStates = struct {
	sync.RWMutex
	state map[int64]string
}{state: make(map[int64]string)}

var bot *tgbotapi.BotAPI

type Queue struct {
	mutex    sync.Mutex
	Position int64
	Users    map[int64]string `json:"users"`
}

var msg tgbotapi.MessageConfig

var queues = struct {
	sync.Mutex
	m map[string]*Queue
}{m: make(map[string]*Queue)}

func queuesToFile(fileName string) error {
	data, err := json.MarshalIndent(queues.m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(fileName, data, 0644) //0644 access rights
}

func queueFromFile(fileName string) error {
	data, err := os.ReadFile(fileName)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &queues.m)
}

func init() {
	if err := godotenv.Load(".env"); err != nil {
		log.Print("No .env file found")
	}
	if err := queueFromFile("s"); err != nil {
		log.Fatal(err)
	}
}

func conTelegram() *tgbotapi.BotAPI {
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TG_BOT_TOKEN"))
	if err != nil {
		panic("Failed to connect to Telegram: " + err.Error())
	}
	return bot
}

func queueToStr() string {
	str := ""
	queues.Lock()
	for i, q := range queues.m {
		str += fmt.Sprintf("%s:\n", i)
		for num, s := range q.Users {
			str += fmt.Sprintf("	%d) %s\n", num, s)
		}
		str += "\n"
	}
	queues.Unlock()
	return str
}

func main() {

	bot = conTelegram()
	updateConfig := tgbotapi.NewUpdate(0)
	queueName := ""

	//queues.m["338 очередь"] = &Queue{Position: 2, Users: map[int64]string{1: "Костя", 2: "Илья"}}
	//queues.m["2 очередь"] = &Queue{Position: 2, Users: map[int64]string{1: "Ебланчик1", 2: "Кобанчик2"}}
	//
	//if err := queuesToFile("s"); err != nil {
	//	log.Fatal(err)
	//}

	for update := range bot.GetUpdatesChan(updateConfig) {
		if update.Message == nil {
			continue
		}

		chatID := update.Message.Chat.ID

		userStates.RLock()
		state := userStates.state[chatID]
		userStates.RUnlock()

		switch state {
		case "awaitingDelete":
			if update.Message.Text == "Вернуться назад" {
				msg = tgbotapi.NewMessage(chatID, queueToStr())
				msg.ReplyMarkup = numericKeyboard
				if _, err := bot.Send(msg); err != nil {
					log.Panic(err)
				}
				userStates.Lock()
				delete(userStates.state, chatID)
				userStates.Unlock()
				continue
			}
			queueName = update.Message.Text
			queues.Lock()
			if _, exists := queues.m[queueName]; exists {
				delete(queues.m, queueName)
				if err := queuesToFile("s"); err != nil {
					log.Fatal(err)
				}
				msg = tgbotapi.NewMessage(chatID, fmt.Sprintf("Очередь \"%s\" удалена", queueName))
				msg.ReplyMarkup = numericKeyboard
				if _, err := bot.Send(msg); err != nil {
					log.Panic(err)
				}
			} else {
				msg = tgbotapi.NewMessage(chatID, "Видимо такой очереди уже нет")
				msg.ReplyMarkup = numericKeyboard
				if _, err := bot.Send(msg); err != nil {
					log.Panic(err)
				}
			}
			queues.Unlock()
			userStates.Lock()
			delete(userStates.state, chatID)
			userStates.Unlock()
			continue
		case "awaitingNaming":
			queues.Lock()
			if _, exists := queues.m[update.Message.Text]; exists {
				msg = tgbotapi.NewMessage(chatID, "Очередь уже существует")

			} else {
				queues.m[update.Message.Text] = &Queue{Users: map[int64]string{}}
				msg = tgbotapi.NewMessage(chatID, "Новая очередь создана")
			}
			queues.Unlock()
			userStates.Lock()
			delete(userStates.state, chatID)
			userStates.Unlock()
			if err := queuesToFile("s"); err != nil {
				log.Fatal(err)
			}
			msg.ReplyMarkup = numericKeyboard
			if _, err := bot.Send(msg); err != nil {
				log.Panic(err)
			}
			continue
		case "awaitingEnter":
			if update.Message.Text == "Вернуться назад" {
				msg = tgbotapi.NewMessage(chatID, queueToStr())
				msg.ReplyMarkup = numericKeyboard
				if _, err := bot.Send(msg); err != nil {
					log.Panic(err)
				}
				userStates.Lock()
				delete(userStates.state, chatID)
				userStates.Unlock()
				continue
			}
			queues.Lock()
			if _, exists := queues.m[update.Message.Text]; exists {
				queueName = update.Message.Text
				userStates.Lock()
				userStates.state[chatID] = "awaitingEnter2"
				userStates.Unlock()
				msg = tgbotapi.NewMessage(chatID, "Введи свое имя")
				msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
			} else {
				msg = tgbotapi.NewMessage(chatID, "Такой очереди нет")
				userStates.Lock()
				delete(userStates.state, chatID)
				userStates.Unlock()
				msg.ReplyMarkup = numericKeyboard
			}
			queues.Unlock()
			if _, err := bot.Send(msg); err != nil {
				log.Panic(err)
			}
			continue
		case "awaitingEnter2":
			queues.Lock()
			if _, exists := queues.m[queueName]; exists {
				queues.m[queueName].Position++
				queues.m[queueName].Users[queues.m[queueName].Position] = update.Message.Text
				if err := queuesToFile("s"); err != nil {
					log.Fatal(err)
				}
				userStates.Lock()
				delete(userStates.state, chatID)
				userStates.Unlock()
				msg = tgbotapi.NewMessage(chatID, "Записан")
				msg.ReplyMarkup = numericKeyboard
			} else {
				msg = tgbotapi.NewMessage(chatID, "Такой очереди нет")
				userStates.Lock()
				delete(userStates.state, chatID)
				userStates.Unlock()
				msg.ReplyMarkup = numericKeyboard
			}
			queues.Unlock()
			if _, err := bot.Send(msg); err != nil {
				log.Panic(err)
			}
			continue
		}

		switch update.Message.Text {
		case "/start":
			str := "Привет, вот очереди которые есть:\n"
			msg = tgbotapi.NewMessage(chatID, str)
			if _, err := bot.Send(msg); err != nil {
				log.Panic(err)
			}
			msg = tgbotapi.NewMessage(chatID, queueToStr())
			msg.ReplyMarkup = numericKeyboard
		case "Удалить очередь":
			msg = tgbotapi.NewMessage(chatID, "Какую очередь удалить?")
			var qKeyboardRows [][]tgbotapi.KeyboardButton
			queues.Lock()
			for i := range queues.m {
				qKeyboardRows = append(qKeyboardRows, tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton(i)))
			}
			queues.Unlock()
			qKeyboardRows = append(qKeyboardRows, tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton("Вернуться назад")))
			var qKeyboard = tgbotapi.NewReplyKeyboard(qKeyboardRows...)
			msg.ReplyMarkup = qKeyboard
			userStates.Lock()
			userStates.state[chatID] = "awaitingDelete"
			userStates.Unlock()
		case "Новая очередь":
			msg = tgbotapi.NewMessage(chatID, "Введите название")
			userStates.Lock()
			userStates.state[chatID] = "awaitingNaming"
			userStates.Unlock()
			msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
		case "Показать списки":
			msg = tgbotapi.NewMessage(chatID, queueToStr())
			msg.ReplyMarkup = numericKeyboard
		case "Войти в очередь":
			msg = tgbotapi.NewMessage(chatID, "В какую очередь встать?")
			var qKeyboardRows [][]tgbotapi.KeyboardButton
			queues.Lock()
			for i := range queues.m {
				qKeyboardRows = append(qKeyboardRows, tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton(i)))
			}
			queues.Unlock()
			qKeyboardRows = append(qKeyboardRows, tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton("Вернуться назад")))
			var qKeyboard = tgbotapi.NewReplyKeyboard(qKeyboardRows...)
			msg.ReplyMarkup = qKeyboard
			userStates.Lock()
			userStates.state[chatID] = "awaitingEnter"
			userStates.Unlock()
		default:
			msg = tgbotapi.NewMessage(chatID, "И?")
		}

		if _, err := bot.Send(msg); err != nil {
			log.Panic(err)
		}
	}
}
