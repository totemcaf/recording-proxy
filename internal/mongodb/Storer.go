package mongodb

import (
	"context"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"net/http"
	"recording-proxy/internal"
	"time"
)

type Config struct {
	Pod            string
	Service        string
	Uri            string
	DatabaseName   string
	Username       string
	Password       string
	CollectionName string
}
type Data struct {
	Headers http.Header `bson:"headers"`
	Body    string      `bson:"body"`
}

type Log struct {
	Id         uuid.UUID `bson:"_id"`
	Pod        string    `bson:"pod"`
	Service    string    `bson:"service"`
	Url        string    `bson:"url"`
	Request    Data      `bson:"request"`
	Response   Data      `bson:"response"`
	Time       time.Time `bson:"time"`
	ElapsedUs  int64     `bson:"elapsed_ms"` // microseconds
	StatusCode int       `bson:"status_code"`
	Format     string    `bson:"format"` // json or str
}

type Storer struct {
	Pod     string
	Service string
	client  *mongo.Client
	logs    *mongo.Collection
	logsCh  chan *internal.RequestResponse
}

func NewStorer(ctx context.Context, config Config) (*Storer, error) {
	log.Println("Creating mongodb storer")
	clientOptions := options.Client().
		ApplyURI(config.Uri).
		SetAuth(options.Credential{
			Username: config.Username,
			Password: config.Password,
		})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, err
	}

	db := client.Database(config.DatabaseName)

	storer := Storer{
		Pod:     config.Pod,
		Service: config.Service,
		client:  client,
		logs:    db.Collection(config.CollectionName),
		logsCh:  make(chan *internal.RequestResponse, 1000),
	}

	if err := storer.checkAccess(ctx); err != nil {
		return nil, err
	}

	return &storer, nil
}

func (s *Storer) Start() {
	go s.proceed()
}

func (s *Storer) Close() {
	close(s.logsCh)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := s.client.Disconnect(ctx)
	if err != nil {
		log.Println("Error disconnecting from mongodb: ", err)
	}
}

func logFromModel(pod string, service string, rr *internal.RequestResponse) *Log {
	return &Log{
		Id:      uuid.New(),
		Pod:     pod,
		Service: service,
		Url:     rr.Url.String(),
		Request: Data{
			Headers: rr.Request.Headers,
			Body:    toBson(rr.Request.Body),
		},
		Response: Data{
			Headers: rr.Response.Headers,
			Body:    toBson(rr.Response.Body),
		},
		Time:       rr.Start,
		ElapsedUs:  rr.End.Sub(rr.Start).Microseconds(),
		StatusCode: rr.StatusCode,
		Format:     "str",
	}
}

func toBson(body []byte) string {
	// Usar Extendend JSON para convertir a bson, si falla mandar como un error con el original
	return string(body)
}

func (s *Storer) Handle(rr *internal.RequestResponse) {
	s.logsCh <- rr
}

func (s *Storer) proceed() {

	for {
		select {
		case rr, ok := <-s.logsCh:
			if !ok {
				return
			}
			s.save(rr)
		}
	}
}

func (s *Storer) save(rr *internal.RequestResponse) { // TODO: Save at request and complete at response?
	l := logFromModel(s.Pod, s.Service, rr)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := s.logs.InsertOne(ctx, l); err != nil {
		log.Println("Error saving log: ", err)
	}
}

func (s *Storer) checkAccess(ctx context.Context) error {
	if err := s.client.Ping(ctx, nil); err != nil {
		return err
	}

	return nil
}
