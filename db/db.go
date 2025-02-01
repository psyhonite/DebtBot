package db

import (
	"log"
	"time"

	"DebtBot/config"
	"DebtBot/models"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3" // Импорт драйвера SQLite
)

type DB struct {
	*sqlx.DB
}

func NewDB(cfg *config.Config) *DB {
	// SQLite connection string is just the database file path
	connStr := cfg.DBName // Используем DBName из конфига как путь к файлу SQLite
	if connStr == "" {
		connStr = "debtbot.db" // Default SQLite file name if not provided in config
	}

	database, err := sqlx.Connect("sqlite3", connStr)
	if err != nil {
		log.Fatalf("Could not connect to database: %v", err)
	}
	log.Println("Successfully connected to database!")
	return &DB{database}
}

// Инициализация таблиц (если их нет)
func (d *DB) InitSchema() error {
	_, err := d.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY, -- BIGINT becomes INTEGER for SQLite, PRIMARY KEY implies AUTOINCREMENT
			created_at DATETIME DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now'))
		);

		CREATE TABLE IF NOT EXISTS credits (
			id INTEGER PRIMARY KEY AUTOINCREMENT, -- SERIAL PRIMARY KEY becomes INTEGER PRIMARY KEY AUTOINCREMENT
			user_id INTEGER REFERENCES users(id), -- BIGINT becomes INTEGER for SQLite
			bank_name TEXT NOT NULL,
			loan_amount DECIMAL NOT NULL, -- DECIMAL should work in SQLite, or you can use REAL/NUMERIC
			due_date DATE NOT NULL,
			created_at DATETIME DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now'))
		);
	`)
	return err
}

// Получение пользователя по ID
func (d *DB) GetUser(userID int64) (*models.User, error) {
	user := &models.User{}
	err := d.Get(user, "SELECT * FROM users WHERE id = ?", userID) // Используем ? для параметров в SQLite
	if err != nil {
		return nil, err
	}
	return user, nil
}

// Создание пользователя, если его нет
func (d *DB) CreateUserIfNotExist(userID int64) (*models.User, error) {
	user, err := d.GetUser(userID)
	if err == nil && user != nil { // Пользователь уже существует
		return user, nil
	}

	_, err = d.Exec("INSERT INTO users (id) VALUES (?)", userID) // Используем ? для параметров в SQLite
	if err != nil {
		return nil, err
	}
	return d.GetUser(userID) // Получаем созданного пользователя
}

// Добавление кредита
func (d *DB) AddCredit(credit *models.Credit) error {
	_, err := d.NamedExec(`
		INSERT INTO credits (user_id, bank_name, loan_amount, due_date)
		VALUES (:user_id, :bank_name, :loan_amount, :due_date)
	`, credit)
	return err
}

// Получение кредитов пользователя
func (d *DB) GetCreditsByUser(userID int64) ([]*models.Credit, error) {
	credits := []*models.Credit{}
	log.Printf("DB.GetCreditsByUser: Запрос кредитов для userID: %d", userID) // <--- Добавили лог
	err := d.Select(&credits, "SELECT * FROM credits WHERE user_id = ? ORDER BY due_date ASC", userID)
	if err != nil {
		log.Printf("DB.GetCreditsByUser: Ошибка при выполнении запроса: %v", err) // <--- Добавили лог ошибки
		return nil, err
	}
	log.Printf("DB.GetCreditsByUser: Найдено кредитов: %d", len(credits)) // <--- Добавили лог количества найденных кредитов
	return credits, nil
}

// Получение кредитов с датой платежа завтра
func (d *DB) GetCreditsDueTomorrow() ([]*models.Credit, error) {
	credits := []*models.Credit{}
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	err := d.Select(&credits, "SELECT * FROM credits WHERE due_date = ?", tomorrow) // Используем ? для параметров в SQLite
	if err != nil {
		return nil, err
	}
	return credits, nil
}

// Удаление кредита по ID
func (d *DB) DeleteCredit(creditID int) error {
	_, err := d.Exec("DELETE FROM credits WHERE id = ?", creditID) // Используем ? для параметров в SQLite
	return err
}
