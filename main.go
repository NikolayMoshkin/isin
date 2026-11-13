package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"opensource.tbank.ru/invest/invest-go/investgo"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("использование: go run . ISIN")
	}
	isin := strings.ToUpper(strings.TrimSpace(os.Args[1]))
	if len(isin) != 12 {
		return fmt.Errorf("ISIN должен содержать 12 символов")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("чтение .env: %w", err)
	}

	token := strings.TrimSpace(os.Getenv("TOKEN"))
	if token == "" {
		return fmt.Errorf("задайте TOKEN в окружении или в .env")
	}

	// Создаём конфигурацию SDK.
	// Token — токен Invest API.
	config := investgo.Config{
		Token:    token,
		AppName:  "my-app",
		EndPoint: "invest-public-api.tbank.ru:443",
	}

	// Создаём логгер.
	logger, err := zap.NewDevelopment()
	if err != nil {
		return err
	}

	// Создаём клиента Invest API.
	client, err := investgo.NewClient(ctx, config, logger.Sugar())
	if err != nil {
		return err
	}

	// Не забываем закрыть gRPC-соединение при завершении программы.
	defer client.Stop()

	// Получаем сервис инструментов.
	instruments := client.NewInstrumentsServiceClient()

	// Ищем инструмент по ISIN.
	response, err := instruments.FindInstrument(isin)
	if err != nil {
		return err
	}

	// FindInstrument возвращает список найденных инструментов.
	found := false
	instrument := response.GetInstruments()[0]

	found = true
	bondResponse, err := instruments.BondByUid(instrument.GetUid())
	if err != nil {
		return fmt.Errorf("получение облигации %s: %w", isin, err)
	}
	bond := bondResponse.GetInstrument()
	if bond == nil {
		return fmt.Errorf("API вернул пустые сведения об облигации %s", isin)
	}
	fmt.Println("Название:", instrument.GetName())
	fmt.Println("ISIN:", instrument.GetIsin())
	fmt.Println("Ticker:", instrument.GetTicker())
	fmt.Println("FIGI:", instrument.GetFigi())
	fmt.Println("UID:", instrument.GetUid())
	fmt.Println("ClassCode:", instrument.GetClassCode())
	fmt.Println("Валюта:", bond.GetCurrency())
	fmt.Println("Купонов в год:", bond.GetCouponQuantityPerYear())
	fmt.Println("Амортизация долга:", yesNo(bond.GetAmortizationFlag()))
	fmt.Println("Плавающий купон (признак API):", yesNo(bond.GetFloatingCouponFlag()))
	if maturity := bond.GetMaturityDate(); maturity != nil {
		fmt.Println("Дата погашения:", maturity.AsTime().Format("2006-01-02"))
	}
	// RiskLevel_RISK_LEVEL_UNSPECIFIED RiskLevel = 0 //Не указан.
	// RiskLevel_RISK_LEVEL_LOW         RiskLevel = 1 //Низкий уровень риска.
	// RiskLevel_RISK_LEVEL_MODERATE    RiskLevel = 2 //Средний уровень риска.
	// RiskLevel_RISK_LEVEL_HIGH        RiskLevel = 3 //Высокий уровень риска.
	fmt.Println("Уровень риска Т-Банка (не кредитный рейтинг):", bond.GetRiskLevel())
	// fmt.Println("Уровень риска клиента:", bond.GetDlong())
	fmt.Println("---")
	if !found {
		return fmt.Errorf("облигация с ISIN %s не найдена в Invest API", isin)
	}
	return nil
}

func yesNo(value bool) string {
	if value {
		return "да"
	}
	return "нет"
}
