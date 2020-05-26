package mongo

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/shellhub-io/shellhub/api/pkg/store"
	"github.com/shellhub-io/shellhub/pkg/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Store struct {
	db *mongo.Database

	store.Store
}

func NewStore(db *mongo.Database) *Store {
	return &Store{db: db}
}

func (s *Store) ListDevices(ctx context.Context, perPage, page int, filters []models.Filter) ([]models.Device, int, error) {
	skip := perPage * (page - 1)

	query := queryDevices()

	// Apply filters if any
	var queryFilter []bson.M
	for _, filter := range filters {
		if filter.Type != "property" {
			continue
		}

		switch filter.Params.Operator {
		case "like":
			queryFilter = append(queryFilter, bson.M{
				filter.Params.Name: bson.M{
					"$regex":   filter.Params.Value,
					"$options": "i",
				},
			})
		case "eq":
			queryFilter = append(queryFilter, bson.M{
				filter.Params.Name: bson.M{
					"$eq": filter.Params.Value,
				},
			})
		case "bool":
			operator, err := strconv.ParseBool(filter.Params.Value)
			if err != nil {
				return nil, 0, err
			}
			queryFilter = append(queryFilter, bson.M{
				filter.Params.Name: bson.M{
					"$eq": operator,
				},
			})
		}
	}

	if len(queryFilter) > 0 {
		query = append(query, bson.M{
			"$match": bson.M{"$or": queryFilter},
		})
	}

	// Only match for the respective tenant if requested
	if tenant := store.TenantFromContext(ctx); tenant != nil {
		query = append(query, bson.M{
			"$match": bson.M{
				"tenant_id": tenant.ID,
			},
		})
	}

	queryCount := append(query, bson.M{"$count": "count"})
	count, err := aggregateCount(ctx, s.db.Collection("devices"), queryCount)
	if err != nil {
		return nil, 0, err
	}

	query = append(query, bson.M{

		"$skip": skip,
	})
	query = append(query, bson.M{

		"$limit": perPage,
	})

	devices := make([]models.Device, 0)

	cursor, err := s.db.Collection("devices").Aggregate(ctx, query)
	if err != nil {
		return devices, count, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		device := new(models.Device)
		err = cursor.Decode(&device)
		if err != nil {
			return devices, count, err
		}
		devices = append(devices, *device)
	}

	return devices, count, err
}

func (s *Store) GetDevice(ctx context.Context, uid models.UID) (*models.Device, error) {
	query := append(queryDevices(), bson.M{
		"$match": bson.M{"uid": uid},
	})

	// Only match for the respective tenant if requested
	if tenant := store.TenantFromContext(ctx); tenant != nil {
		query = append(query, bson.M{
			"$match": bson.M{
				"tenant_id": tenant.ID,
			},
		})
	}

	device := new(models.Device)

	cursor, err := s.db.Collection("devices").Aggregate(ctx, query)
	defer cursor.Close(ctx)
	cursor.Next(ctx)

	err = cursor.Decode(&device)
	if err != nil {
		return nil, err
	}

	return device, nil
}

func (s *Store) DeleteDevice(ctx context.Context, uid models.UID) error {
	if _, err := s.db.Collection("devices").DeleteOne(ctx, bson.M{"uid": uid}); err != nil {
		return err
	}

	if _, err := s.db.Collection("sessions").DeleteOne(ctx, bson.M{"device": uid}); err != nil {
		return err
	}

	return nil
}

func (s *Store) AddDevice(ctx context.Context, d models.Device) error {
	hostname := strings.Replace(d.Identity.MAC, ":", "-", -1)

	q := bson.M{
		"$setOnInsert": bson.M{
			"name": hostname,
		},
		"$set": d,
	}
	opts := options.Update().SetUpsert(true)
	_, err := s.db.Collection("devices").UpdateOne(ctx, bson.M{"uid": d.UID}, q, opts)
	return err
}

func (s *Store) RenameDevice(ctx context.Context, uid models.UID, name string) error {
	if _, err := s.db.Collection("devices").UpdateOne(ctx, bson.M{"uid": uid}, bson.M{"$set": bson.M{"name": name}}); err != nil {
		return err
	}
	return nil
}

func (s *Store) LookupDevice(ctx context.Context, namespace, name string) (*models.Device, error) {
	user := new(models.User)
	if err := s.db.Collection("users").FindOne(ctx, bson.M{"username": namespace}).Decode(&user); err != nil {
		return nil, err
	}

	device := new(models.Device)
	if err := s.db.Collection("devices").FindOne(ctx, bson.M{"tenant_id": user.TenantID, "name": name}).Decode(&device); err != nil {
		return nil, err
	}

	return device, nil
}

func (s *Store) UpdateDeviceStatus(ctx context.Context, uid models.UID, online bool) error {
	device := new(models.Device)
	if err := s.db.Collection("devices").FindOne(ctx, bson.M{"uid": uid}).Decode(&device); err != nil {
		return err
	}

	if !online {
		_, err := s.db.Collection("connected_devices").DeleteMany(ctx, bson.M{"uid": uid})
		return err
	}
	device.LastSeen = time.Now()
	opts := options.Update().SetUpsert(true)
	_, err := s.db.Collection("devices").UpdateOne(ctx, bson.M{"uid": device.UID}, bson.M{"$set": bson.M{"last_seen": device.LastSeen}}, opts)
	if err != nil {
		return err
	}

	cd := &models.ConnectedDevice{
		UID:      device.UID,
		TenantID: device.TenantID,
		LastSeen: time.Now(),
	}
	if _, err := s.db.Collection("connected_devices").InsertOne(ctx, &cd); err != nil {
		return err
	}

	return nil
}

func (s *Store) ListSessions(ctx context.Context, perPage, page int) ([]models.Session, int, error) {
	skip := perPage * (page - 1)
	query := []bson.M{
		{
			"$sort": bson.M{
				"started_at": -1,
			},
		},

		{
			"$lookup": bson.M{
				"from":         "active_sessions",
				"localField":   "uid",
				"foreignField": "uid",
				"as":           "active",
			},
		},
		{
			"$addFields": bson.M{
				"active": bson.M{"$anyElementTrue": []interface{}{"$active"}},
			},
		},
	}

	// Only match for the respective tenant if requested
	if tenant := store.TenantFromContext(ctx); tenant != nil {
		query = append(query, bson.M{
			"$match": bson.M{
				"tenant_id": tenant.ID,
			},
		})
	}

	queryCount := append(query, bson.M{"$count": "count"})
	count, err := aggregateCount(ctx, s.db.Collection("sessions"), queryCount)
	if err != nil {
		return nil, 0, err
	}

	query = append(query, bson.M{

		"$skip": skip,
	})
	query = append(query, bson.M{

		"$limit": perPage,
	})

	sessions := make([]models.Session, 0)
	cursor, err := s.db.Collection("sessions").Aggregate(ctx, query)
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		session := new(models.Session)
		err = cursor.Decode(&session)
		if err != nil {
			return sessions, count, err
		}

		device, err := s.GetDevice(ctx, session.DeviceUID)
		if err != nil {
			return sessions, count, err
		}

		session.Device = device
		sessions = append(sessions, *session)
	}

	return sessions, count, err
}

func (s *Store) GetSession(ctx context.Context, uid models.UID) (*models.Session, error) {
	query := []bson.M{
		{
			"$match": bson.M{"uid": uid},
		},
		{
			"$lookup": bson.M{
				"from":         "active_sessions",
				"localField":   "uid",
				"foreignField": "uid",
				"as":           "active",
			},
		},
		{
			"$addFields": bson.M{
				"active": bson.M{"$anyElementTrue": []interface{}{"$active"}},
			},
		},
	}

	// Only match for the respective tenant if requested
	if tenant := store.TenantFromContext(ctx); tenant != nil {
		query = append(query, bson.M{
			"$match": bson.M{
				"tenant_id": tenant.ID,
			},
		})
	}

	session := new(models.Session)

	cursor, err := s.db.Collection("sessions").Aggregate(ctx, query)
	defer cursor.Close(ctx)
	cursor.Next(ctx)

	err = cursor.Decode(&session)
	if err != nil {
		return nil, err
	}

	device, err := s.GetDevice(ctx, session.DeviceUID)
	if err != nil {
		return nil, err
	}

	session.Device = device

	return session, nil
}

func (s *Store) SetSessionAuthenticated(ctx context.Context, uid models.UID, authenticated bool) error {
	_, err := s.db.Collection("sessions").UpdateOne(ctx, bson.M{"uid": uid}, bson.M{"$set": bson.M{"authenticated": authenticated}})
	return err
}

func (s *Store) CreateSession(ctx context.Context, session models.Session) (*models.Session, error) {
	session.StartedAt = time.Now()
	session.LastSeen = session.StartedAt

	device, err := s.GetDevice(ctx, session.DeviceUID)
	if err != nil {
		return nil, err
	}

	session.TenantID = device.TenantID

	if _, err := s.db.Collection("sessions").InsertOne(ctx, &session); err != nil {
		return nil, err
	}

	as := &models.ActiveSession{
		UID:      models.UID(session.UID),
		LastSeen: session.StartedAt,
	}

	if _, err := s.db.Collection("active_sessions").InsertOne(ctx, &as); err != nil {
		return nil, err
	}

	return &session, nil
}

func (s *Store) GetStats(ctx context.Context) (*models.Stats, error) {
	query := []bson.M{
		{"$group": bson.M{"_id": bson.M{"uid": "$uid"}, "count": bson.M{"$sum": 1}}},
		{"$group": bson.M{"_id": bson.M{"uid": "$uid"}, "count": bson.M{"$sum": 1}}},
	}

	// Only match for the respective tenant if requested
	if tenant := store.TenantFromContext(ctx); tenant != nil {
		query = append([]bson.M{{
			"$match": bson.M{
				"tenant_id": tenant.ID,
			},
		}}, query...)
	}

	onlineDevices, err := aggregateCount(ctx, s.db.Collection("connected_devices"), query)
	if err != nil {
		return nil, err
	}

	query = []bson.M{
		{"$count": "count"},
	}

	// Only match for the respective tenant if requested
	if tenant := store.TenantFromContext(ctx); tenant != nil {
		query = append([]bson.M{{
			"$match": bson.M{
				"tenant_id": tenant.ID,
			},
		}}, query...)
	}

	registeredDevices, err := aggregateCount(ctx, s.db.Collection("devices"), query)
	if err != nil {
		return nil, err
	}

	query = []bson.M{
		{
			"$lookup": bson.M{
				"from":         "active_sessions",
				"localField":   "uid",
				"foreignField": "uid",
				"as":           "active",
			},
		},
		{
			"$addFields": bson.M{
				"active": bson.M{"$anyElementTrue": []interface{}{"$active"}},
			},
		},
		{
			"$match": bson.M{
				"active": true,
			},
		},
	}

	// Only match for the respective tenant if requested
	if tenant := store.TenantFromContext(ctx); tenant != nil {
		query = append(query, bson.M{
			"$match": bson.M{
				"tenant_id": tenant.ID,
			},
		})
	}

	query = append(query, bson.M{
		"$count": "count",
	})

	activeSessions, err := aggregateCount(ctx, s.db.Collection("sessions"), query)
	if err != nil {
		return nil, err
	}

	return &models.Stats{
		RegisteredDevices: registeredDevices,
		OnlineDevices:     onlineDevices,
		ActiveSessions:    activeSessions,
	}, nil
}

func (s *Store) KeepAliveSession(ctx context.Context, uid models.UID) error {
	session := models.Session{}

	err := s.db.Collection("sessions").FindOne(ctx, bson.M{"uid": uid}).Decode(&session)
	if err != nil {
		return err
	}

	session.LastSeen = time.Now()

	opts := options.Update().SetUpsert(true)
	_, err = s.db.Collection("sessions").UpdateOne(ctx, bson.M{"uid": session.UID}, bson.M{"$set": session}, opts)
	if err != nil {
		return err
	}

	activeSession := &models.ActiveSession{
		UID:      uid,
		LastSeen: time.Now(),
	}

	if _, err := s.db.Collection("active_sessions").InsertOne(ctx, &activeSession); err != nil {
		return err
	}

	return nil
}

func (s *Store) DeactivateSession(ctx context.Context, uid models.UID) error {
	session := new(models.Session)
	if err := s.db.Collection("sessions").FindOne(ctx, bson.M{"uid": uid}).Decode(&session); err != nil {
		return err
	}

	session.LastSeen = time.Now()
	opts := options.Update().SetUpsert(true)
	_, err := s.db.Collection("sessions").UpdateOne(ctx, bson.M{"uid": session.UID}, bson.M{"$set": session}, opts)
	if err != nil {
		return err
	}

	_, err = s.db.Collection("active_sessions").DeleteMany(ctx, bson.M{"uid": session.UID})
	return err
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	user := new(models.User)

	if err := s.db.Collection("users").FindOne(ctx, bson.M{"username": username}).Decode(&user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *Store) GetUserByTenant(ctx context.Context, tenant string) (*models.User, error) {
	user := new(models.User)
	if err := s.db.Collection("users").FindOne(ctx, bson.M{"tenant_id": tenant}).Decode(&user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Store) GetDeviceByMac(ctx context.Context, mac, tenant string) (*models.Device, error) {
	device := new(models.Device)
	if err := s.db.Collection("devices").FindOne(ctx, bson.M{"tenant_id": tenant, "identity": bson.M{"mac": mac}}).Decode(&device); err != nil {
		return nil, err
	}

	return device, nil
}

func (s *Store) GetDeviceByName(ctx context.Context, name, tenant string) (*models.Device, error) {
	device := new(models.Device)
	if err := s.db.Collection("devices").FindOne(ctx, bson.M{"tenant_id": tenant, "name": name}).Decode(&device); err != nil {
		return nil, err
	}

	return device, nil
}

func (s *Store) GetDeviceByUID(ctx context.Context, uid models.UID, tenant string) (*models.Device, error) {
	device := new(models.Device)
	if err := s.db.Collection("devices").FindOne(ctx, bson.M{"tenant_id": tenant, "uid": uid}).Decode(&device); err != nil {
		return nil, err
	}

	return device, nil
}

func EnsureIndexes(db *mongo.Database) error {
	mod := mongo.IndexModel{
		Keys:    bson.D{{"uid", 1}},
		Options: options.Index().SetName("uid").SetUnique(true),
	}
	_, err := db.Collection("devices").Indexes().CreateOne(context.TODO(), mod)
	if err != nil {
		return err
	}

	mod = mongo.IndexModel{
		Keys:    bson.D{{"last_seen", 1}},
		Options: options.Index().SetName("last_seen").SetExpireAfterSeconds(30),
	}
	_, err = db.Collection("connected_devices").Indexes().CreateOne(context.TODO(), mod)
	if err != nil {
		return err
	}

	mod = mongo.IndexModel{
		Keys:    bson.D{{"uid", 1}},
		Options: options.Index().SetName("uid").SetUnique(false),
	}
	_, err = db.Collection("connected_devices").Indexes().CreateOne(context.TODO(), mod)
	if err != nil {
		return err
	}

	mod = mongo.IndexModel{
		Keys:    bson.D{{"uid", 1}},
		Options: options.Index().SetName("uid").SetUnique(true),
	}
	_, err = db.Collection("sessions").Indexes().CreateOne(context.TODO(), mod)
	if err != nil {
		return err
	}

	mod = mongo.IndexModel{
		Keys:    bson.D{{"last_seen", 1}},
		Options: options.Index().SetName("last_seen").SetExpireAfterSeconds(30),
	}
	_, err = db.Collection("active_sessions").Indexes().CreateOne(context.TODO(), mod)
	if err != nil {
		return err
	}

	mod = mongo.IndexModel{
		Keys:    bson.D{{"uid", 1}},
		Options: options.Index().SetName("uid").SetUnique(false),
	}
	_, err = db.Collection("active_sessions").Indexes().CreateOne(context.TODO(), mod)
	if err != nil {
		return err
	}

	mod = mongo.IndexModel{
		Keys:    bson.D{{"username", 1}},
		Options: options.Index().SetName("username").SetUnique(true),
	}
	_, err = db.Collection("users").Indexes().CreateOne(context.TODO(), mod)
	if err != nil {
		return err
	}

	mod = mongo.IndexModel{
		Keys:    bson.D{{"tenant_id", 1}},
		Options: options.Index().SetName("tenant_id").SetUnique(true),
	}
	_, err = db.Collection("users").Indexes().CreateOne(context.TODO(), mod)
	if err != nil {
		return err
	}
	return nil
}

func queryDevices() []bson.M {
	return []bson.M{
		{

			"$lookup": bson.M{
				"from":         "connected_devices",
				"localField":   "uid",
				"foreignField": "uid",
				"as":           "online",
			},
		},
		{
			"$lookup": bson.M{
				"from":         "users",
				"localField":   "tenant_id",
				"foreignField": "tenant_id",
				"as":           "namespace",
			},
		},
		{
			"$addFields": bson.M{
				"online":    bson.M{"$anyElementTrue": []interface{}{"$online"}},
				"namespace": "$namespace.username",
			},
		},
		{
			"$unwind": "$namespace",
		},
	}
}
