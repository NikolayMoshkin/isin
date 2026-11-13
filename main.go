package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"opensource.tbank.ru/invest/invest-go/investgo"
	investapi "opensource.tbank.ru/invest/invest-go/proto"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

type SheetData struct {
	OK   bool `json:"ok"`
	Data []struct {
		Row  int    `json:"row"`
		Isin string `json:"isin"`
	} `json:"data"`
}

type UpdateResp struct {
	Isin           string `json:"isin"`
	Rating         string `json:"rating"`
	CouponsPerYear int    `json:"couponsPerYear"`
	Comment        string `json:"comment"`
}

type postEnvelope struct {
	Updates []*UpdateResp `json:"updates"`
}

type postResult struct {
	OK      bool            `json:"ok"`
	Error   string          `json:"error,omitempty"`
	Results json.RawMessage `json:"results,omitempty"`
}

func run() error {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("чтение .env: %w", err)
	}

	url := strings.TrimSpace(os.Getenv("ENDPOINT"))
	if url == "" {
		return fmt.Errorf("задайте ENDPOINT в окружении или в .env")
	}

	sheetData, err := getSheetData(url)
	if err != nil {
		return err
	}

	respList, err := getIsinInfo(sheetData)
	if err != nil {
		return err
	}

	if err := postSheetUpdates(url, respList); err != nil {
		return err
	}

	fmt.Printf("обновлено записей: %d\n", len(respList))
	return nil
}

func getIsinInfo(d *SheetData) ([]*UpdateResp, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	token := strings.TrimSpace(os.Getenv("TOKEN"))
	if token == "" {
		return nil, fmt.Errorf("задайте TOKEN в окружении или в .env")
	}

	config := investgo.Config{
		Token:    token,
		AppName:  "my-app",
		EndPoint: "invest-public-api.tbank.ru:443",
	}

	logger, err := zap.NewDevelopment()
	if err != nil {
		return nil, err
	}

	client, err := investgo.NewClient(ctx, config, logger.Sugar())
	if err != nil {
		return nil, err
	}
	defer client.Stop()

	instruments := client.NewInstrumentsServiceClient()
	updateRespList := make([]*UpdateResp, 0, len(d.Data))

	for _, data := range d.Data {
		isin := strings.ToUpper(strings.TrimSpace(data.Isin))
		if len(isin) != 12 {
			return nil, fmt.Errorf("ISIN должен содержать 12 символов: %q", isin)
		}

		response, err := instruments.FindInstrument(isin)
		if err != nil {
			return nil, fmt.Errorf("поиск %s: %w", isin, err)
		}
		found := response.GetInstruments()
		if len(found) == 0 {
			return nil, fmt.Errorf("облигация с ISIN %s не найдена в Invest API", isin)
		}
		instrument := found[0]

		bondResponse, err := instruments.BondByUid(instrument.GetUid())
		if err != nil {
			return nil, fmt.Errorf("получение облигации %s: %w", isin, err)
		}
		bond := bondResponse.GetInstrument()
		if bond == nil {
			return nil, fmt.Errorf("API вернул пустые сведения об облигации %s", isin)
		}

		var specs []string
		if bond.GetFloatingCouponFlag() {
			specs = append(specs, "Флоатер")
		}
		if bond.GetAmortizationFlag() {
			specs = append(specs, "Амортизация")
		}

		rating := ""
		switch bond.GetRiskLevel() {
		case investapi.RiskLevel_RISK_LEVEL_LOW:
			rating = "Высокий"
		case investapi.RiskLevel_RISK_LEVEL_MODERATE:
			rating = "Средний"
		case investapi.RiskLevel_RISK_LEVEL_HIGH:
			rating = "Низкий"
		}

		updateRespList = append(updateRespList, &UpdateResp{
			Isin:           isin,
			Rating:         rating,
			CouponsPerYear: int(bond.GetCouponQuantityPerYear()),
			Comment:        strings.Join(specs, ";"),
		})
	}

	return updateRespList, nil
}

func getSheetData(url string) (*SheetData, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d: %s", url, resp.StatusCode, truncate(body, 200))
	}

	var r SheetData
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("разбор ответа GET: %w; body=%s", err, truncate(body, 200))
	}
	if !r.OK && len(r.Data) == 0 {
		return nil, fmt.Errorf("GET вернул пустые данные: %s", truncate(body, 200))
	}
	return &r, nil
}

// Apps Script doPost ждёт {"updates":[...]}, а не голый массив.
// Редирект 302 → GET на echo-URL обрабатывается стандартным http.Client.
func postSheetUpdates(url string, updates []*UpdateResp) error {
	payload, err := json.Marshal(postEnvelope{Updates: updates})
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("POST: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("чтение ответа POST: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("POST status %d: %s", resp.StatusCode, truncate(body, 300))
	}

	var result postResult
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("разбор ответа POST: %w; body=%s", err, truncate(body, 300))
	}
	if !result.OK {
		return fmt.Errorf("Apps Script ошибка: %s", result.Error)
	}
	return nil
}

func truncate(b []byte, n int) string {
	s := string(b)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
