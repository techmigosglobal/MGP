package masterdata

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type Store struct{ DB *sql.DB }

type InventoryItem struct {
	ID, Code, Name, Category, SerialNo, BatchNo, Unit, Holder, Description, Status string
	Quantity                                                                       string
}
type Consignee struct{ ID, Name, Address, Contact, Status string }

func (s Store) ImportInventory(ctx context.Context, items []InventoryItem) error {
	if len(items) == 0 {
		return fmt.Errorf("CSV contains no inventory rows")
	}
	for _, item := range items {
		if strings.TrimSpace(item.Code) == "" || strings.TrimSpace(item.Name) == "" {
			return fmt.Errorf("every row needs an item code and name")
		}
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, item := range items {
		if _, err := tx.ExecContext(ctx, `INSERT INTO inventory_items(item_code,item_name,category,unit_of_measure,quantity,holder) VALUES($1,$2,$3,$4,$5,$6)`, strings.TrimSpace(item.Code), strings.TrimSpace(item.Name), item.Category, item.Unit, item.Quantity, item.Holder); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s Store) Inventory(ctx context.Context) ([]InventoryItem, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id,item_code,item_name,category,serial_no,batch_no,unit_of_measure,quantity::text,holder,description,CASE WHEN archived_at IS NULL THEN 'active' ELSE 'archived' END FROM inventory_items ORDER BY archived_at NULLS FIRST,item_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []InventoryItem
	for rows.Next() {
		var item InventoryItem
		if err := rows.Scan(&item.ID, &item.Code, &item.Name, &item.Category, &item.SerialNo, &item.BatchNo, &item.Unit, &item.Quantity, &item.Holder, &item.Description, &item.Status); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s Store) CreateInventory(ctx context.Context, item InventoryItem) (string, error) {
	if strings.TrimSpace(item.Code) == "" || strings.TrimSpace(item.Name) == "" {
		return "", fmt.Errorf("item code and name are required")
	}
	var id string
	err := s.DB.QueryRowContext(ctx, `INSERT INTO inventory_items(item_code,item_name,category,serial_no,batch_no,unit_of_measure,quantity,holder,description) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`, strings.TrimSpace(item.Code), strings.TrimSpace(item.Name), item.Category, item.SerialNo, item.BatchNo, item.Unit, item.Quantity, item.Holder, item.Description).Scan(&id)
	return id, err
}

func (s Store) ArchiveInventory(ctx context.Context, id string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE inventory_items SET archived_at=now(),updated_at=now() WHERE id=$1 AND archived_at IS NULL`, id)
	return err
}

func (s Store) Consignees(ctx context.Context) ([]Consignee, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id,name,address,contact,CASE WHEN archived_at IS NULL THEN 'active' ELSE 'archived' END FROM consignees ORDER BY archived_at NULLS FIRST,name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Consignee
	for rows.Next() {
		var item Consignee
		if err := rows.Scan(&item.ID, &item.Name, &item.Address, &item.Contact, &item.Status); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s Store) CreateConsignee(ctx context.Context, item Consignee) (string, error) {
	if strings.TrimSpace(item.Name) == "" {
		return "", fmt.Errorf("consignee name is required")
	}
	var id string
	err := s.DB.QueryRowContext(ctx, `INSERT INTO consignees(name,address,contact) VALUES($1,$2,$3) RETURNING id`, strings.TrimSpace(item.Name), item.Address, item.Contact).Scan(&id)
	return id, err
}

func (s Store) ArchiveConsignee(ctx context.Context, id string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE consignees SET archived_at=now(),updated_at=now() WHERE id=$1 AND archived_at IS NULL`, id)
	return err
}
