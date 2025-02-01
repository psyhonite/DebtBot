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

	log.Println("Начинаем обработку обновлений...")

	for update := range updates {
		log.Println("Получено обновление:", update)

		if update.Message != nil { // Handle messages
			log.Println("Обновление содержит сообщение:", update.Message)

			userID := int64(update.Message.From.ID)
			b.db.CreateUserIfNotExist(userID) // Ensure user exists in DB

			command := update.Message.Command()
			text := update.Message.Text

			log.Printf("Команда: '%s', Текст: '%s'", command, text)

			switch command {
			case "start", "help":
				log.Println("Команда: /start или /help")
				b.handleHelpCommand(update.Message)
			case "addcredit":
				log.Println("Команда: /addcredit")
				b.handleAddCreditCommand(update.Message)
			case "mycredits":
				log.Println("Команда: /mycredits")
				b.handleMyCreditsCommand(update.Message)
			case "deletecredit":
				log.Println("Команда: /deletecredit")
				b.handleDeleteCreditCommand(update.Message)
			default:
				// Check for button presses (text messages from reply keyboard)
				switch text {
				case "Добавить кредит":
					log.Println("Кнопка: Добавить кредит")
					b.handleAddCreditCommand(update.Message)
				case "Мои кредиты":
					log.Println("Кнопка: Мои кредиты")
					b.handleMyCreditsCommand(update.Message)
				case "Удалить кредит":
					log.Println("Кнопка: Удалить кредит")
					b.handleDeleteCreditCommand(update.Message)
				case "Помощь":
					log.Println("Кнопка: Помощь")
					b.handleHelpCommand(update.Message)
				default:
					log.Println("Команда не распознана, проверяем состояние пользователя")
					// Обработка ввода данных в процессе добавления кредита
					if state, ok := b.state[userID]; ok {
						log.Printf("Состояние пользователя %d найдено: %s, вызов handleInputData", userID, state)
						b.handleInputData(update.Message, state)
					} else if !strings.HasPrefix(text, "/") { // Ignore non-command messages after command flow
						log.Println("Состояние не найдено и это не команда, отправляем 'Неизвестная команда'")

					} else {
						log.Println("Состояние не найдено, но это команда (начинается с /), игнорируем")
					}
				}
			}
		} else {
			log.Println("Обновление без сообщения, пропускаем")
			continue
		}
	}
	return nil
}

// handleCallbackQuery больше не нужен, т.к. используем ReplyKeyboard

func (b *Bot) handleHelpCommand(message *tgbotapi.Message) {
	helpText := `
Привет! Я бот для учета твоих кредитов.

Выберите действие:`

	// Create ReplyKeyboardMarkup
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("➕ Добавить кредит"),
			tgbotapi.NewKeyboardButton("💶 Мои кредиты"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("➖ Удалить кредит"),
			tgbotapi.NewKeyboardButton("🆘 Помощь"),
		),
	)
	keyboard.ResizeKeyboard = true // Optional: make keyboard smaller

	msg := tgbotapi.NewMessage(message.Chat.ID, helpText)
	msg.ReplyMarkup = keyboard
	msg.ParseMode = tgbotapi.ModeMarkdown
	_, err := b.botAPI.Send(msg)
	if err != nil {
		log.Printf("Error sending message with buttons: %v", err)
	}
	b.sendMessage(message.Chat.ID, helpText, message.MessageID) // Corrected sendMessage call
}

// Новая функция-обертка для handleAddCreditCommand, принимающая UserID как аргумент
func (b *Bot) handleAddCreditCommandForCallback(message *tgbotapi.Message, userID int64) {
	log.Printf("handleAddCreditCommandForCallback - UserID из callbackQuery.From.ID: %d", userID) // ЛОГ для новой функции
	b.state[userID] = "waiting_bank_name"
	b.inputData[userID] = make(map[string]string)
	log.Printf("Состояние для пользователя %d установлено в: %s", userID, b.state[userID])

	msgText := "Введите название банка:"
	msg := tgbotapi.NewMessage(message.Chat.ID, msgText)

	// Optionally add a cancel button if needed during input process - Inline Keyboard still possible if needed for cancel
	/*cancelKeyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Отмена", "cancel_addcredit"),
		),
	)
	msg.ReplyMarkup = cancelKeyboard*/

	_, err := b.botAPI.Send(msg)
	if err != nil {
		log.Printf("Error sending message: %v", err)
	}
}

// handleAddCreditCommand теперь вызывается ТОЛЬКО при получении текстовой команды /addcredit
// и использует message.From.ID как и раньше
func (b *Bot) handleAddCreditCommand(message *tgbotapi.Message) {
	userID := int64(message.From.ID)
	log.Printf("handleAddCreditCommand (текстовая команда/кнопка) - UserID из message.From.ID: %d", userID) // ЛОГ для текстовой команды и кнопок
	b.state[userID] = "waiting_bank_name"
	b.inputData[userID] = make(map[string]string)
	log.Printf("Состояние для пользователя %d установлено в: %s", userID, b.state[userID])

	msgText := "Введите название банка:"
	b.sendMessage(message.Chat.ID, msgText, message.MessageID)
}

func (b *Bot) handleInputData(message *tgbotapi.Message, state string) {
	userID := int64(message.From.ID)
	text := message.Text
	log.Printf("handleInputData вызвана для пользователя %d, состояние: %s, текст: %s", userID, state, text)

	switch state {
	case "waiting_bank_name":
		b.inputData[userID]["bank_name"] = text
		b.state[userID] = "waiting_loan_amount"
		log.Printf("Состояние пользователя %d изменено на: %s, банк: %s", userID, b.state[userID], text)
		b.sendMessage(message.Chat.ID, "Введите сумму кредита:", message.MessageID)

	case "waiting_loan_amount":
		_, err := strconv.ParseFloat(text, 64)
		if err != nil {
			b.sendMessage(message.Chat.ID, "Некорректная сумма. Введите число, например, 10000.50", message.MessageID)
			return
		}
		b.inputData[userID]["loan_amount"] = text
		b.state[userID] = "waiting_due_date"
		log.Printf("Состояние пользователя %d изменено на: %s, сумма: %s", userID, b.state[userID], text)
		b.sendMessage(message.Chat.ID, "Введите дату платежа в формате ГГГГ-ММ-ДД (например, 2024-12-31):", message.MessageID)

	case "waiting_due_date":
		_, err := time.Parse("2006-01-02", text)
		if err != nil {
			b.sendMessage(message.Chat.ID, "Некорректный формат даты. Используйте ГГГГ-ММ-ДД (например, 2024-12-31)", message.MessageID)
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
			b.sendMessage(message.Chat.ID, "Ошибка при сохранении кредита. Попробуйте еще раз.", message.MessageID)
		} else {
			b.sendMessage(message.Chat.ID, "Кредит успешно добавлен!", message.MessageID)
		}

		delete(b.state, userID)
		delete(b.inputData, userID)
		log.Printf("Состояние и данные пользователя %d сброшены", userID)

	case "waiting_credit_to_delete": // <--- Обработка выбора кредита для удаления
		creditIndex, err := strconv.Atoi(text)
		if err != nil {
			b.sendMessage(message.Chat.ID, "Пожалуйста, введите номер кредита для удаления.", message.MessageID)
			return
		}

		creditsToDelete, ok := b.inputData[userID]["credits_to_delete"]
		if !ok {
			log.Printf("Error: credits_to_delete data not found for user %d", userID)
			b.sendMessage(message.Chat.ID, "Произошла ошибка, попробуйте еще раз.", message.MessageID)
			delete(b.state, userID)
			delete(b.inputData, userID)
			return
		}

		creditIDs := strings.Split(creditsToDelete, ",") // Assuming IDs are stored as comma-separated string
		if creditIndex <= 0 || creditIndex > len(creditIDs) {
			b.sendMessage(message.Chat.ID, "Неверный номер кредита. Пожалуйста, выберите номер из списка.", message.MessageID)
			return
		}

		creditIDToDelete, err := strconv.Atoi(creditIDs[creditIndex-1]) // Get the correct credit ID
		if err != nil {
			log.Printf("Error converting credit ID to int: %v", err)
			b.sendMessage(message.Chat.ID, "Произошла ошибка, попробуйте еще раз.", message.MessageID)
			delete(b.state, userID)
			delete(b.inputData, userID)
			return
		}

		err = b.db.DeleteCredit(creditIDToDelete) // <--- Вызов функции удаления из БД
		if err != nil {
			log.Printf("Error deleting credit from DB: %v", err)
			b.sendMessage(message.Chat.ID, "Ошибка при удалении кредита. Попробуйте еще раз.", message.MessageID)
		} else {
			b.sendMessage(message.Chat.ID, "Кредит успешно удален!", message.MessageID)
		}

		delete(b.state, userID)
		delete(b.inputData, userID)
		log.Printf("Состояние и данные пользователя %d сброшены после удаления кредита", userID)
	}
}

// НОВАЯ функция-обертка для handleMyCreditsCommand, вызываемая из CallbackQuery
func (b *Bot) handleMyCreditsCommandForCallback(message *tgbotapi.Message, userID int64) {
	log.Printf("handleMyCreditsCommandForCallback - UserID из callbackQuery.From.ID: %d", userID) // ЛОГ
	credits, err := b.db.GetCreditsByUser(userID)                                                 // <-- ИСПОЛЬЗУЕМ ПЕРЕДАННЫЙ userID
	if err != nil {
		log.Printf("handleMyCreditsCommandForCallback: Ошибка при получении кредитов из DB: %v", err)
		b.sendMessage(message.Chat.ID, "Ошибка при получении списка кредитов.", message.MessageID)
		return
	}

	if len(credits) == 0 {
		b.sendMessage(message.Chat.ID, "У вас пока нет добавленных кредитов. Используйте /addcredit чтобы добавить.", message.MessageID)
		return
	}

	formattedCredits := "*Ваши кредиты:*\n\n"
	for _, credit := range credits {
		formattedCredits += fmt.Sprintf("🏦 *Банк:* %s\n", credit.BankName)
		formattedCredits += fmt.Sprintf("💰 *Сумма кредита:* %.2f ₽\n ", credit.LoanAmount)
		formattedCredits += fmt.Sprintf("📅 *Дата платежа:* %s\n", credit.DueDate.Format("02.01.2006"))
		formattedCredits += "---\n"
	}

	b.sendMessage(message.Chat.ID, formattedCredits, message.MessageID)

}

// handleMyCreditsCommand теперь вызывается ТОЛЬКО при получении текстовой команды /mycredits
func (b *Bot) handleMyCreditsCommand(message *tgbotapi.Message) {
	userID := int64(message.From.ID)                                                                 // <-- UserID из message.From.ID для текстовой команды
	log.Printf("handleMyCreditsCommand (текстовая команда) - UserID из message.From.ID: %d", userID) // ЛОГ
	credits, err := b.db.GetCreditsByUser(userID)
	if err != nil {
		log.Printf("handleMyCreditsCommand (текстовая команда): Ошибка при получении кредитов из DB: %v", err)
		b.sendMessage(message.Chat.ID, "Ошибка при получении списка кредитов.", message.MessageID)
		return
	}

	if len(credits) == 0 {
		b.sendMessage(message.Chat.ID, "У вас пока нет добавленных кредитов. Используйте /addcredit чтобы добавить.", message.MessageID)
		return
	}

	formattedCredits := "*Ваши кредиты:*\n\n"
	for _, credit := range credits {
		formattedCredits += fmt.Sprintf("🏦 *Банк:* %s\n", credit.BankName)
		formattedCredits += fmt.Sprintf("💰 *Сумма кредита:* %.2f ₽\n ", credit.LoanAmount)
		formattedCredits += fmt.Sprintf("📅 *Дата платежа:* %s\n", credit.DueDate.Format("02.01.2006"))
		formattedCredits += "---\n"
	}

	b.sendMessage(message.Chat.ID, formattedCredits, message.MessageID)
}

// Новая функция-обертка для handleDeleteCreditCommand, вызываемая из CallbackQuery
func (b *Bot) handleDeleteCreditCommandForCallback(message *tgbotapi.Message, userID int64) {
	log.Printf("handleDeleteCreditCommandForCallback - UserID из callbackQuery.From.ID: %d", userID) // ЛОГ
	credits, err := b.db.GetCreditsByUser(userID)
	if err != nil {
		log.Printf("handleDeleteCreditCommandForCallback: Ошибка при получении кредитов из DB: %v", err)
		b.sendMessage(message.Chat.ID, "Ошибка при получении списка кредитов для удаления.", message.MessageID)
		return
	}

	if len(credits) == 0 {
		b.sendMessage(message.Chat.ID, "У вас нет кредитов для удаления. Используйте /addcredit чтобы добавить.", message.MessageID)
		return
	}

	formattedCredits := "Выберите номер кредита для удаления:\n\n"
	var creditIDs []string // To store credit IDs for later deletion
	for i, credit := range credits {
		formattedCredits += fmt.Sprintf("%d. 🏦 *Банк:* %s, 💰 *Сумма кредита:* %.2f ₽, 📅 *Дата платежа:* %s\n", i+1, credit.BankName, credit.LoanAmount, credit.DueDate.Format("02.01.2006"))
		creditIDs = append(creditIDs, strconv.Itoa(credit.ID)) // Store credit IDs as strings
	}

	b.sendMessage(message.Chat.ID, formattedCredits, message.MessageID)
}

// handleDeleteCreditCommand теперь вызывается ТОЛЬКО при получении текстовой команды /deletecredit
func (b *Bot) handleDeleteCreditCommand(message *tgbotapi.Message) {
	userID := int64(message.From.ID)
	log.Printf("handleDeleteCreditCommand (текстовая команда) - UserID из message.From.ID: %d", userID) // ЛОГ
	credits, err := b.db.GetCreditsByUser(userID)
	if err != nil {
		log.Printf("handleDeleteCreditCommand (текстовая команда): Ошибка при получении кредитов из DB: %v", err)
		b.sendMessage(message.Chat.ID, "Ошибка при получении списка кредитов для удаления.", message.MessageID)
		return
	}

	if len(credits) == 0 {
		b.sendMessage(message.Chat.ID, "У вас нет кредитов для удаления. Используйте /addcredit чтобы добавить.", message.MessageID)
		return
	}

	formattedCredits := "Выберите номер кредита для удаления:\n\n"
	var creditIDs []string // To store credit IDs for later deletion
	for i, credit := range credits {
		formattedCredits += fmt.Sprintf("%d. 🏦 *Банк:* %s, 💰 *Сумма кредита:* %.2f ₽, 📅 *Дата платежа:* %s\n", i+1, credit.BankName, credit.LoanAmount, credit.DueDate.Format("02.01.2006"))
		creditIDs = append(creditIDs, strconv.Itoa(credit.ID)) // Store credit IDs as strings
	}

	b.sendMessage(message.Chat.ID, formattedCredits, message.MessageID)
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
		b.sendMessage(user.ID, notificationText, 0) // No reply for notifications
	}
}

// Modified sendMessage function to accept replyToMessageID
func (b *Bot) sendMessage(chatID int64, text string, replyToMessageID int) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown // Включаем Markdown для форматирования
	if replyToMessageID != 0 {
		msg.ReplyToMessageID = replyToMessageID // Set reply to message ID if provided
	}
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
