package bot

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"DebtBot/config"
	"DebtBot/db"
	"DebtBot/models"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type Bot struct {
	botAPI    *tgbotapi.BotAPI
	db        *db.DB
	state     map[int64]string            // Состояние для каждого пользователя (для обработки ввода)
	inputData map[int64]map[string]string // Временные данные ввода для каждого пользователя
}

func NewBot(cfg *config.Config, database *db.DB) (*Bot, error) {
	botAPI, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		return nil, fmt.Errorf("error creating bot API: %w", err)
	}

	return &Bot{
		botAPI:    botAPI,
		db:        database,
		state:     make(map[int64]string),
		inputData: make(map[int64]map[string]string),
	}, nil
}

func (b *Bot) Start() error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := b.botAPI.GetUpdatesChan(u)
	if err != nil {
		return fmt.Errorf("error getting updates channel: %w", err)
	}

	log.Println("Начинаем обработку обновлений...") // <<<---  Важный лог при старте обработки

	for update := range updates {
		log.Println("Получено обновление:", update) // <<<---  Лог каждого обновления

		if update.Message == nil { // Ignore non-message updates
			log.Println("Обновление без сообщения, пропускаем") // <<<--- Лог если нет сообщения
			continue
		}
		log.Println("Обновление содержит сообщение:", update.Message) // <<<--- Лог если есть сообщение

		userID := int64(update.Message.From.ID)
		b.db.CreateUserIfNotExist(userID) // Ensure user exists in DB

		command := update.Message.Command()
		text := update.Message.Text

		log.Printf("Команда: '%s', Текст: '%s'", command, text) // <<<--- Лог команды и текста

		switch command {
		case "start", "help":
			log.Println("Команда: /start или /help") // <<<--- Лог обработки команды /start или /help
			b.handleHelpCommand(update.Message)
		case "addcredit":
			log.Println("Команда: /addcredit") // <<<--- Лог обработки команды /addcredit
			b.handleAddCreditCommand(update.Message)
		case "mycredits":
			log.Println("Команда: /mycredits") // <<<--- Лог обработки команды /mycredits
			b.handleMyCreditsCommand(update.Message)
		case "deletecredit":
			log.Println("Команда: /deletecredit")       // <<<--- Лог обработки команды /deletecredit
			b.handleDeleteCreditCommand(update.Message) // <--- Добавлено обработчик удаления кредита
		default:
			log.Println("Команда не распознана, проверяем состояние пользователя") // <<<--- Лог для default кейса
			// Обработка ввода данных в процессе добавления кредита
			if state, ok := b.state[userID]; ok {
				log.Printf("Состояние пользователя %d найдено: %s, вызов handleInputData", userID, state) // Добавим лог
				b.handleInputData(update.Message, state)
			} else if !strings.HasPrefix(text, "/") { // Ignore non-command messages after command flow
				log.Println("Состояние не найдено и это не команда, отправляем 'Неизвестная команда'") // <<<--- Лог если состояние не найдено и не команда
				b.sendMessage(update.Message.Chat.ID, "Неизвестная команда. Используйте /help для списка команд.")
			} else {
				log.Println("Состояние не найдено, но это команда (начинается с /), игнорируем") // <<<--- Лог если состояние не найдено, но это команда
			}
		}
	}
	return nil
}

func (b *Bot) handleHelpCommand(message *tgbotapi.Message) {
	helpText := `
Привет! Я бот для учета твоих кредитов.

Команды:

/start или /help - Показать это сообщение.
/addcredit - Добавить новый кредит.
/mycredits - Показать список твоих кредитов.
/deletecredit - Удалить кредит.
`
	b.sendMessage(message.Chat.ID, helpText)
}

func (b *Bot) handleAddCreditCommand(message *tgbotapi.Message) {
	userID := int64(message.From.ID)
	b.state[userID] = "waiting_bank_name"
	b.inputData[userID] = make(map[string]string)                                          // Инициализация для нового ввода
	log.Printf("Состояние для пользователя %d установлено в: %s", userID, b.state[userID]) // Добавим лог
	b.sendMessage(message.Chat.ID, "Введите название банка:")
}

func (b *Bot) handleInputData(message *tgbotapi.Message, state string) {
	userID := int64(message.From.ID)
	text := message.Text
	log.Printf("handleInputData вызвана для пользователя %d, состояние: %s, текст: %s", userID, state, text) // Добавим лог в начале

	switch state {
	case "waiting_bank_name":
		b.inputData[userID]["bank_name"] = text
		b.state[userID] = "waiting_loan_amount"
		log.Printf("Состояние пользователя %d изменено на: %s, банк: %s", userID, b.state[userID], text) // Добавим лог
		b.sendMessage(message.Chat.ID, "Введите сумму кредита:")

	case "waiting_loan_amount":
		_, err := strconv.ParseFloat(text, 64)
		if err != nil {
			b.sendMessage(message.Chat.ID, "Некорректная сумма. Введите число, например, 10000.50")
			return
		}
		b.inputData[userID]["loan_amount"] = text
		b.state[userID] = "waiting_due_date"
		log.Printf("Состояние пользователя %d изменено на: %s, сумма: %s", userID, b.state[userID], text) // Добавим лог
		b.sendMessage(message.Chat.ID, "Введите дату платежа в формате ГГГГ-ММ-ДД (например, 2024-12-31):")

	case "waiting_due_date":
		_, err := time.Parse("2006-01-02", text)
		if err != nil {
			b.sendMessage(message.Chat.ID, "Некорректный формат даты. Используйте ГГГГ-ММ-ДД (например, 2024-12-31)")
			return
		}
		b.inputData[userID]["due_date"] = text

		credit := &models.Credit{
			UserID:     userID,
			BankName:   b.inputData[userID]["bank_name"],
			LoanAmount: parseFloat(b.inputData[userID]["loan_amount"]),
			DueDate:    parseDate(b.inputData[userID]["due_date"]),
		}

		err = b.db.AddCredit(credit)
		if err != nil {
			log.Printf("Error adding credit to DB: %v", err)
			b.sendMessage(message.Chat.ID, "Ошибка при сохранении кредита. Попробуйте еще раз.")
		} else {
			b.sendMessage(message.Chat.ID, "Кредит успешно добавлен!")
		}

		delete(b.state, userID)
		delete(b.inputData, userID)
		log.Printf("Состояние и данные пользователя %d сброшены", userID) // Добавим лог

	case "waiting_credit_to_delete": // <--- Обработка выбора кредита для удаления
		creditIndex, err := strconv.Atoi(text)
		if err != nil {
			b.sendMessage(message.Chat.ID, "Пожалуйста, введите номер кредита для удаления.")
			return
		}

		creditsToDelete, ok := b.inputData[userID]["credits_to_delete"]
		if !ok {
			log.Printf("Error: credits_to_delete data not found for user %d", userID)
			b.sendMessage(message.Chat.ID, "Произошла ошибка, попробуйте еще раз.")
			delete(b.state, userID)
			delete(b.inputData, userID)
			return
		}

		creditIDs := strings.Split(creditsToDelete, ",") // Assuming IDs are stored as comma-separated string
		if creditIndex <= 0 || creditIndex > len(creditIDs) {
			b.sendMessage(message.Chat.ID, "Неверный номер кредита. Пожалуйста, выберите номер из списка.")
			return
		}

		creditIDToDelete, err := strconv.Atoi(creditIDs[creditIndex-1]) // Get the correct credit ID
		if err != nil {
			log.Printf("Error converting credit ID to int: %v", err)
			b.sendMessage(message.Chat.ID, "Произошла ошибка, попробуйте еще раз.")
			delete(b.state, userID)
			delete(b.inputData, userID)
			return
		}

		err = b.db.DeleteCredit(creditIDToDelete) // <--- Вызов функции удаления из БД
		if err != nil {
			log.Printf("Error deleting credit from DB: %v", err)
			b.sendMessage(message.Chat.ID, "Ошибка при удалении кредита. Попробуйте еще раз.")
		} else {
			b.sendMessage(message.Chat.ID, "Кредит успешно удален!")
		}

		delete(b.state, userID)
		delete(b.inputData, userID)
		log.Printf("Состояние и данные пользователя %d сброшены после удаления кредита", userID) // Добавим лог
	}
}

func (b *Bot) handleMyCreditsCommand(message *tgbotapi.Message) {
	userID := int64(message.From.ID)
	credits, err := b.db.GetCreditsByUser(userID)
	if err != nil {
		log.Printf("Error getting credits from DB: %v", err)
		b.sendMessage(message.Chat.ID, "Ошибка при получении списка кредитов.")
		return
	}

	if len(credits) == 0 {
		b.sendMessage(message.Chat.ID, "У вас пока нет добавленных кредитов. Используйте /addcredit чтобы добавить.")
		return
	}

	formattedCredits := "*Ваши кредиты:*\n\n"
	for _, credit := range credits {
		formattedCredits += fmt.Sprintf("🏦 *Банк:* %s\n", credit.BankName)
		formattedCredits += fmt.Sprintf("💰 *Сумма кредита:* %.2f ₽\n ", credit.LoanAmount)
		formattedCredits += fmt.Sprintf("📅 *Дата платежа:* %s\n", credit.DueDate.Format("02.01.2006"))
		formattedCredits += "---\n"
	}

	b.sendMessage(message.Chat.ID, formattedCredits)
}

func (b *Bot) handleDeleteCreditCommand(message *tgbotapi.Message) {
	userID := int64(message.From.ID)
	credits, err := b.db.GetCreditsByUser(userID)
	if err != nil {
		log.Printf("Error getting credits from DB: %v", err)
		b.sendMessage(message.Chat.ID, "Ошибка при получении списка кредитов для удаления.")
		return
	}

	if len(credits) == 0 {
		b.sendMessage(message.Chat.ID, "У вас нет кредитов для удаления. Используйте /addcredit чтобы добавить.")
		return
	}

	formattedCredits := "Выберите номер кредита для удаления:\n\n"
	var creditIDs []string // To store credit IDs for later deletion
	for i, credit := range credits {
		formattedCredits += fmt.Sprintf("%d. 🏦 *Банк:* %s, 💰 *Сумма кредита:* %.2f ₽, 📅 *Дата платежа:* %s\n", i+1, credit.BankName, credit.LoanAmount, credit.DueDate.Format("02.01.2006"))
		creditIDs = append(creditIDs, strconv.Itoa(credit.ID)) // Store credit IDs as strings
	}

	b.inputData[userID] = make(map[string]string)
	b.inputData[userID]["credits_to_delete"] = strings.Join(creditIDs, ",") // Store comma-separated IDs
	b.state[userID] = "waiting_credit_to_delete"
	b.sendMessage(message.Chat.ID, formattedCredits)
}

func (b *Bot) SendNotifications() {
	credits, err := b.db.GetCreditsDueTomorrow()
	if err != nil {
		log.Printf("Error getting credits due tomorrow: %v", err)
		return
	}

	for _, credit := range credits {
		user, err := b.db.GetUser(credit.UserID)
		if err != nil {
			log.Printf("Error getting user %d: %v", credit.UserID, err)
			continue
		}

		notificationText := fmt.Sprintf("🔔 *Напоминание о платеже по кредиту!*\n\nБанк: %s\nСумма: %.2f\nДата платежа: %s\n\nНе забудьте оплатить кредит завтра!",
			credit.BankName, credit.LoanAmount, credit.DueDate.Format("02.01.2006"))
		b.sendMessage(user.ID, notificationText)
	}
}

func (b *Bot) sendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown // Включаем Markdown для форматирования
	_, err := b.botAPI.Send(msg)
	if err != nil {
		log.Printf("Error sending message: %v", err)
	}
}

// Вспомогательные функции для парсинга
func parseFloat(s string) float64 {
	val, _ := strconv.ParseFloat(s, 64) // Игнорируем ошибку, т.к. валидация была раньше
	return val
}

func parseDate(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s) // Игнорируем ошибку, т.к. валидация была раньше
	return t
}
