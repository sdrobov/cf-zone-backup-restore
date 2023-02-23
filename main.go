package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/urfave/cli/v2"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type (
	zoneOwner struct {
		Id    string `json:"id"`
		Email string `json:"email"`
		Type  string `json:"type"`
	}
	zoneAccount struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	}
	zonePlan struct {
		Id           string `json:"id"`
		Name         string `json:"name"`
		Price        int    `json:"price"`
		Currency     string `json:"currency"`
		Frequency    string `json:"frequency"`
		LegacyId     string `json:"legacy_id"`
		IsSubscribed bool   `json:"is_subscribed"`
		CanSubscribe bool   `json:"can_subscribe"`
	}
	zonePlanPending struct {
		Id           string `json:"id"`
		Name         string `json:"name"`
		Price        int    `json:"price"`
		Currency     string `json:"currency"`
		Frequency    string `json:"frequency"`
		LegacyId     string `json:"legacy_id"`
		IsSubscribed bool   `json:"is_subscribed"`
		CanSubscribe bool   `json:"can_subscribe"`
	}
	zone struct {
		Id                  string           `json:"id"`
		Name                string           `json:"name"`
		DevelopmentMode     int              `json:"development_mode"`
		OriginalNameServers []string         `json:"original_name_servers"`
		OriginalRegistrar   string           `json:"original_registrar"`
		OriginalDnshost     string           `json:"original_dnshost"`
		CreatedOn           time.Time        `json:"created_on"`
		ModifiedOn          time.Time        `json:"modified_on"`
		ActivatedOn         time.Time        `json:"activated_on"`
		Owner               *zoneOwner       `json:"owner"`
		Account             *zoneAccount     `json:"account"`
		Permissions         []string         `json:"permissions"`
		Plan                *zonePlan        `json:"plan"`
		PlanPending         *zonePlanPending `json:"plan_pending"`
		Status              string           `json:"status"`
		Paused              bool             `json:"paused"`
		Type                string           `json:"type"`
		NameServers         []string         `json:"name_servers"`
	}
	listZonesResponse struct {
		Success  bool          `json:"success"`
		Errors   []interface{} `json:"errors"`
		Messages []interface{} `json:"messages"`
		Result   []*zone       `json:"result"`
	}
	zoneDnsRecordMeta struct {
		AutoAdded bool   `json:"auto_added"`
		Source    string `json:"source"`
	}
	zoneDnsRecord struct {
		Id         string             `json:"id"`
		Type       string             `json:"type"`
		Name       string             `json:"name"`
		Content    string             `json:"content"`
		Proxiable  bool               `json:"proxiable"`
		Proxied    bool               `json:"proxied"`
		Comment    string             `json:"comment"`
		Tags       []string           `json:"tags"`
		Ttl        int                `json:"ttl"`
		Locked     bool               `json:"locked"`
		ZoneId     string             `json:"zone_id"`
		ZoneName   string             `json:"zone_name"`
		CreatedOn  time.Time          `json:"created_on"`
		ModifiedOn time.Time          `json:"modified_on"`
		Meta       *zoneDnsRecordMeta `json:"meta"`
	}
	listZoneDnsRecordsResponse struct {
		Success  bool             `json:"success"`
		Errors   []interface{}    `json:"errors"`
		Messages []interface{}    `json:"messages"`
		Result   []*zoneDnsRecord `json:"result"`
	}
)

func main() {
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "email",
				Aliases:  []string{"e"},
				Usage:    "cf account email address",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "key",
				Aliases:  []string{"k"},
				Usage:    "cf account api key",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "token",
				Aliases:  []string{"t"},
				Usage:    "cf account api token",
				Required: false,
			},
			&cli.StringFlag{
				Name:        "dir",
				Aliases:     []string{"d"},
				Usage:       "backup directory",
				Required:    false,
				DefaultText: "./",
			},
			&cli.StringFlag{
				Name:        "url",
				Aliases:     []string{"u"},
				Usage:       "cf api url",
				Required:    false,
				DefaultText: "https://api.cloudflare.com/client/v4/",
			},
			&cli.BoolFlag{
				Name:        "verbose",
				Aliases:     []string{"v"},
				Usage:       "verbose output",
				Required:    false,
				DefaultText: "false",
			},
		},
		Commands: []*cli.Command{
			{
				Name:   "backup",
				Usage:  "backup zones",
				Action: backupZones,
			},
			{
				Name:   "restore",
				Usage:  "restore zones",
				Action: restoreZones,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(fmt.Errorf("error running app: %w", err))
	}
}

func restoreZones(ctx *cli.Context) error {
	email := ctx.String("email")
	key := ctx.String("key")
	token := ctx.String("token")
	if email == "" && key == "" && token == "" {
		return fmt.Errorf("email & key or token must be provided")
	}
	dir := ctx.String("dir")
	url := ctx.String("url")
	if url == "" {
		url = "https://api.cloudflare.com/client/v4/"
	}
	verbose := ctx.Bool("verbose")

	backupFile := filepath.Join(dir, "backup.json")
	backup, err := os.Open(backupFile)
	if err != nil {
		return fmt.Errorf("error opening backup file: %w", err)
	}
	defer backup.Close()

	decoder := json.NewDecoder(backup)
	var dnsRecords []*zoneDnsRecord
	err = decoder.Decode(&dnsRecords)
	if err != nil {
		return fmt.Errorf("error decoding backup file: %w", err)
	}

	if verbose {
		fmt.Printf("restoring %d dns records\n", len(dnsRecords))
	}

	for i, dnsRecord := range dnsRecords {
		if verbose {
			fmt.Printf("restoring dns record %s; %d/%d\n", dnsRecord.Name, i+1, len(dnsRecords))
		}
		body := &bytes.Buffer{}
		err := json.NewEncoder(body).Encode(dnsRecord)
		if err != nil {
			return fmt.Errorf("error encoding dns record: %w", err)
		}
		client := &http.Client{}
		req, err := http.NewRequest("PUT", url+"zones/"+dnsRecord.ZoneId+"/dns_records/"+dnsRecord.Id, body)
		if err != nil {
			return fmt.Errorf("error creating request: %w", err)
		}

		if email != "" && key != "" {
			req.Header.Add("X-Auth-Email", email)
			req.Header.Add("X-Auth-Key", key)
		} else {
			req.Header.Add("Authorization", "Bearer "+token)
		}
		req.Header.Add("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("error sending request: %w", err)
		}
		resp.Body.Close()
	}

	return nil
}

func backupZones(ctx *cli.Context) error {
	email := ctx.String("email")
	key := ctx.String("key")
	token := ctx.String("token")
	if email == "" && key == "" && token == "" {
		return fmt.Errorf("email & key or token must be provided")
	}
	dir := ctx.String("dir")
	url := ctx.String("url")
	if url == "" {
		url = "https://api.cloudflare.com/client/v4/"
	}
	verbose := ctx.Bool("verbose")

	client := &http.Client{}
	req, err := http.NewRequest("GET", url+"zones", nil)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	if email != "" && key != "" {
		req.Header.Add("X-Auth-Email", email)
		req.Header.Add("X-Auth-Key", key)
	} else {
		req.Header.Add("Authorization", "Bearer "+token)
	}
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)
	var response listZonesResponse
	err = decoder.Decode(&response)
	if err != nil {
		return fmt.Errorf("error decoding response: %w", err)
	}

	if !response.Success {
		return fmt.Errorf("error response: %v", response.Errors)
	}

	if verbose {
		fmt.Printf("Found %d zones\n", len(response.Result))
	}

	forBackup := make([]*zoneDnsRecord, 0, len(response.Result))
	for i, zone := range response.Result {
		if verbose {
			fmt.Printf("Backing up zone %s; %d/%d\n", zone.Name, i+1, len(response.Result))
		}
		req, err := http.NewRequest("GET", url+"zones/"+zone.Id+"/dns_records", nil)
		if err != nil {
			return fmt.Errorf("error creating request: %w", err)
		}

		if email != "" && key != "" {
			req.Header.Add("X-Auth-Email", email)
			req.Header.Add("X-Auth-Key", key)
		} else {
			req.Header.Add("Authorization", "Bearer "+token)
		}
		req.Header.Add("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("error sending request: %w", err)
		}
		defer resp.Body.Close()

		decoder := json.NewDecoder(resp.Body)
		var response listZoneDnsRecordsResponse
		err = decoder.Decode(&response)
		if err != nil {
			return fmt.Errorf("error decoding response: %w", err)
		}

		if !response.Success {
			return fmt.Errorf("error response: %v", response.Errors)
		}

		forBackup = append(forBackup, response.Result...)
	}

	backupFile, err := os.Create(dir + "backup.json")
	if err != nil {
		return fmt.Errorf("error creating backup file: %w", err)
	}
	defer backupFile.Close()

	encoder := json.NewEncoder(backupFile)
	err = encoder.Encode(forBackup)
	if err != nil {
		return fmt.Errorf("error encoding backup file: %w", err)
	}

	return nil
}
