package common

import (
	"github.com/go-redis/redis"
	"strconv"
	"github.com/spf13/cast"
)

const KEY_UID_CS = "uid:cs:"

type RedisOp struct {
	redisClient *redis.Client
}

func NewRedisOp(addr string) *RedisOp {
	redisClient := newRedisClient(addr)
	return &RedisOp{
		redisClient: redisClient,
	}
}

// addr redis://user:password@localhost:6379/0
func newRedisClient(addr string) *redis.Client {
	opt, err := redis.ParseURL(addr)
	if err != nil {
		panic(err)
	}
	client := redis.NewClient(opt)
	_, err = client.Ping().Result()

	if err != nil {
		panic(err)
	}

	return client
}

func (redis RedisOp) GetCsId(uid uint64) (int, error) {
	csId, err := redis.redisClient.Get(buildKey(uid)).Result()
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(csId)
}

func (redis RedisOp) SetCsId(uid uint64, csId int) error {
	_, err := redis.redisClient.Set(buildKey(uid), csId, 0).Result()
	return err
}

func (redis RedisOp) DelCsId(uid uint64) error {
	_, err := redis.redisClient.Del(buildKey(uid)).Result()
	return err
}

func buildKey(uid uint64) string {
	return KEY_UID_CS + cast.ToString(uid)
}
