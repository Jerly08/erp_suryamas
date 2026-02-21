package main

import (
	"fmt"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/models"
)

func main() {
	config.LoadConfig()
	db := database.ConnectDB()

	var acc169 models.Account
	db.Where("id = ?", 169).First(&acc169)
	fmt.Printf("Account 169: Code=%s, Name=%s, Type=%s\n", acc169.Code, acc169.Name, acc169.Type)

	var childAccounts []models.Account
	db.Where("code LIKE '1102%'").Find(&childAccounts)
	for _, a := range childAccounts {
		fmt.Printf("1102* Account: ID=%d, Code=%s, Name=%s, Type=%s\n", a.ID, a.Code, a.Name, a.Type)
	}

	var code1201 struct {
		ID   uint64
		Code string
		Name string
	}
	db.Raw("SELECT id, code, name FROM accounts WHERE code LIKE '1201%'").Scan(&code1201)
	fmt.Printf("1201* Account: ID=%d, Code=%s, Name=%s\n", code1201.ID, code1201.Code, code1201.Name)
}
