package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/domain"
)

type Dynamo struct {
	api   *dynamodb.Client
	table string
}

func NewDynamo(api *dynamodb.Client, table string) *Dynamo {
	return &Dynamo{api: api, table: table}
}

type record struct {
	PK     string `dynamodbav:"pk"`
	SK     string `dynamodbav:"sk"`
	GSI1PK string `dynamodbav:"gsi1pk,omitempty"`
	GSI1SK string `dynamodbav:"gsi1sk,omitempty"`
	Entity string `dynamodbav:"entity"`
	Body   string `dynamodbav:"body"`
}

func (d *Dynamo) put(ctx context.Context, rec record) error {
	item, err := attributevalue.MarshalMap(rec)
	if err != nil {
		return err
	}
	_, err = d.api.PutItem(ctx, &dynamodb.PutItemInput{TableName: aws.String(d.table), Item: item})
	return err
}

func (d *Dynamo) get(ctx context.Context, pk, sk string, dest any) error {
	out, err := d.api.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(d.table),
		Key:            key(pk, sk),
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return err
	}
	if out.Item == nil {
		return ErrNotFound
	}
	var rec record
	if err := attributevalue.UnmarshalMap(out.Item, &rec); err != nil {
		return err
	}
	return json.Unmarshal([]byte(rec.Body), dest)
}

func (d *Dynamo) PutProduct(ctx context.Context, p domain.Product) error {
	if p.Reservations == nil {
		p.Reservations = map[string]int{}
	}
	return d.putJSON(ctx, "PRODUCT#"+p.ID, "META", "CATALOG", "PRODUCT#"+p.ID, "product", p)
}

func (d *Dynamo) GetProduct(ctx context.Context, id string) (domain.Product, error) {
	var p domain.Product
	if err := d.get(ctx, "PRODUCT#"+id, "META", &p); err != nil {
		return domain.Product{}, err
	}
	if p.Reservations == nil {
		p.Reservations = map[string]int{}
	}
	return p, nil
}

func (d *Dynamo) ListProducts(ctx context.Context) ([]domain.Product, error) {
	out, err := d.api.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(d.table),
		IndexName:              aws.String("gsi1"),
		KeyConditionExpression: aws.String("gsi1pk = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: "CATALOG"},
		},
	})
	if err != nil {
		return nil, err
	}
	products := make([]domain.Product, 0, len(out.Items))
	for _, item := range out.Items {
		var rec record
		if err := attributevalue.UnmarshalMap(item, &rec); err != nil {
			return nil, err
		}
		var p domain.Product
		if err := json.Unmarshal([]byte(rec.Body), &p); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, nil
}

func (d *Dynamo) CreateUser(ctx context.Context, u domain.User) error {
	return d.putJSON(ctx, "USER#"+u.ID, "META", "", "", "user", u)
}

func (d *Dynamo) GetUser(ctx context.Context, id string) (domain.User, error) {
	var u domain.User
	if err := d.get(ctx, "USER#"+id, "META", &u); err != nil {
		return domain.User{}, err
	}
	return u, nil
}

func (d *Dynamo) CreateSession(ctx context.Context, s domain.Session) error {
	return d.putJSON(ctx, "SESSION#"+s.ID, "META", "", "", "session", s)
}

func (d *Dynamo) GetSession(ctx context.Context, id string) (domain.Session, error) {
	var s domain.Session
	if err := d.get(ctx, "SESSION#"+id, "META", &s); err != nil {
		return domain.Session{}, err
	}
	if time.Now().UTC().After(s.ExpiresAt) {
		return domain.Session{}, ErrSessionExpired
	}
	return s, nil
}

func (d *Dynamo) PutCart(ctx context.Context, c domain.Cart) error {
	return d.putJSON(ctx, "CART#"+c.ID, "META", "USER#"+c.UserID, "CART#"+c.ID, "cart", c)
}

func (d *Dynamo) GetCart(ctx context.Context, id string) (domain.Cart, error) {
	var c domain.Cart
	if err := d.get(ctx, "CART#"+id, "META", &c); err != nil {
		return domain.Cart{}, err
	}
	return c, nil
}

func (d *Dynamo) GetOpenCartByUser(ctx context.Context, userID string) (domain.Cart, error) {
	out, err := d.api.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(d.table),
		IndexName:              aws.String("gsi1"),
		KeyConditionExpression: aws.String("gsi1pk = :pk AND begins_with(gsi1sk, :sk)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: "USER#" + userID},
			":sk": &types.AttributeValueMemberS{Value: "CART#"},
		},
	})
	if err != nil {
		return domain.Cart{}, err
	}
	var found domain.Cart
	ok := false
	for _, item := range out.Items {
		var rec record
		if err := attributevalue.UnmarshalMap(item, &rec); err != nil {
			return domain.Cart{}, err
		}
		var c domain.Cart
		if err := json.Unmarshal([]byte(rec.Body), &c); err != nil {
			return domain.Cart{}, err
		}
		if c.IsTerminal() {
			continue
		}
		if !ok || c.CreatedAt.After(found.CreatedAt) {
			found = c
			ok = true
		}
	}
	if !ok {
		return domain.Cart{}, ErrNotFound
	}
	return found, nil
}

func (d *Dynamo) PutOrder(ctx context.Context, o domain.Order) error {
	sk := "ORDER#" + o.CreatedAt.UTC().Format(time.RFC3339Nano) + "#" + o.ID
	return d.putJSON(ctx, "ORDER#"+o.ID, "META", "USER#"+o.UserID, sk, "order", o)
}

func (d *Dynamo) GetOrder(ctx context.Context, id string) (domain.Order, error) {
	var o domain.Order
	if err := d.get(ctx, "ORDER#"+id, "META", &o); err != nil {
		return domain.Order{}, err
	}
	return o, nil
}

func (d *Dynamo) ListOrdersByUser(ctx context.Context, userID string) ([]domain.Order, error) {
	out, err := d.api.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(d.table),
		IndexName:              aws.String("gsi1"),
		KeyConditionExpression: aws.String("gsi1pk = :pk AND begins_with(gsi1sk, :sk)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: "USER#" + userID},
			":sk": &types.AttributeValueMemberS{Value: "ORDER#"},
		},
		ScanIndexForward: aws.Bool(false),
	})
	if err != nil {
		return nil, err
	}
	orders := make([]domain.Order, 0, len(out.Items))
	for _, item := range out.Items {
		var rec record
		if err := attributevalue.UnmarshalMap(item, &rec); err != nil {
			return nil, err
		}
		var o domain.Order
		if err := json.Unmarshal([]byte(rec.Body), &o); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, nil
}

func (d *Dynamo) Reserve(ctx context.Context, cartID, productID string, qty int) (domain.Product, error) {
	if qty == 0 {
		return d.GetProduct(ctx, productID)
	}
	p, err := d.GetProduct(ctx, productID)
	if err != nil {
		return domain.Product{}, err
	}
	if qty > 0 && p.Available < qty {
		return domain.Product{}, ErrInsufficientStock
	}
	if qty < 0 && p.Reservations[cartID] < -qty {
		return domain.Product{}, ErrConflict
	}
	p.Available -= qty
	p.Reserved += qty
	if p.Reservations == nil {
		p.Reservations = map[string]int{}
	}
	p.Reservations[cartID] += qty
	if p.Reservations[cartID] == 0 {
		delete(p.Reservations, cartID)
	}
	if err := d.PutProduct(ctx, p); err != nil {
		return domain.Product{}, err
	}
	return p, nil
}

func (d *Dynamo) ReleaseCart(ctx context.Context, cartID string) error {
	return d.mutateReservations(ctx, cartID, func(p *domain.Product, qty int) {
		p.Available += qty
		p.Reserved -= qty
		delete(p.Reservations, cartID)
	})
}

func (d *Dynamo) CommitCart(ctx context.Context, cartID string) error {
	return d.mutateReservations(ctx, cartID, func(p *domain.Product, qty int) {
		p.Reserved -= qty
		p.Sold += qty
		delete(p.Reservations, cartID)
	})
}

func (d *Dynamo) mutateReservations(ctx context.Context, cartID string, fn func(*domain.Product, int)) error {
	products, err := d.ListProducts(ctx)
	if err != nil {
		return err
	}
	for _, p := range products {
		qty := p.Reservations[cartID]
		if qty == 0 {
			continue
		}
		fresh, err := d.GetProduct(ctx, p.ID)
		if err != nil {
			return err
		}
		held := fresh.Reservations[cartID]
		if held == 0 {
			continue
		}
		fn(&fresh, held)
		if err := d.PutProduct(ctx, fresh); err != nil {
			return err
		}
	}
	return nil
}

func (d *Dynamo) putJSON(ctx context.Context, pk, sk, gsi1pk, gsi1sk, entity string, v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return d.put(ctx, record{
		PK:     pk,
		SK:     sk,
		GSI1PK: gsi1pk,
		GSI1SK: gsi1sk,
		Entity: entity,
		Body:   string(body),
	})
}

func key(pk, sk string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"pk": &types.AttributeValueMemberS{Value: pk},
		"sk": &types.AttributeValueMemberS{Value: sk},
	}
}
