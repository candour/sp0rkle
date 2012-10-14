package urldriver

import (
	"encoding/base64"
	"flag"
	"fmt"
	"github.com/fluffle/golog/logging"
	"github.com/fluffle/sp0rkle/collections/urls"
	"github.com/fluffle/sp0rkle/db"
	"github.com/fluffle/sp0rkle/util"
	"hash/crc32"
	"io"
	"labix.org/v2/mgo/bson"
	"net/http"
	"os"
	"strings"
	"time"
)

const driverName string = "urls"
const shortenPath string = "/s/"
const cachePath string = "/c/"
const autoShortenLimit int = 120

var badUrlStrings = []string{
	"4chan",
}

var urlCacheDir *string = flag.String("url_cache_dir",
	util.JoinPath(os.Getenv("HOME"), ".sp0rkle"),
	"Path to store cached content under.")

type urlDriver struct {
	*urls.UrlCollection
	// Remember the last url seen on a per-channel basis
	lastseen map[string]bson.ObjectId
	l logging.Logger
}

func UrlDriver(db *db.Database, l logging.Logger) *urlDriver {
	return &urlDriver{
		UrlCollection: urls.Collection(db, l),
		lastseen: make(map[string]bson.ObjectId),
		l: l,
	}
}

func (ud *urlDriver) Name() string {
	return driverName
}

func (ud *urlDriver) Encode(url string) string {
	// We shorten/cache a url with it's base-64 encoded CRC32 hash
	crc := crc32.ChecksumIEEE([]byte(url))
	crcb := make([]byte, 4)
	for i := 0; i < 4; i++ {
		crcb[i] = byte((crc>>uint32(i)) & 0xff)
	}
	// Avoid collisions in shortened URLs
	for i := 0; i < 10; i++ {
		// Since we're always encoding exactly 4 bytes (32 bits)
		// resulting in 5 1/3 bytes of encoded data, we can drop
		// the two padding equals signs for brevity.
		s := (base64.URLEncoding.EncodeToString(crcb))[:6]
		q := ud.Find(bson.M{"$or": []bson.M{
			bson.M{"cachedas": s}, bson.M{"shortened": s}}})
		if n, err := q.Count(); n == 0 && err == nil {
			return s
		}
		crcb[util.RNG.Intn(4)]++
	}
	ud.l.Warn("Collided ten times while encoding URL.")
	return ""
}

func (ud *urlDriver) Shorten(u *urls.Url) error {
	u.Shortened = ud.Encode(u.Url)
	if _, err := ud.Upsert(bson.M{"url": u.Url}, u); err != nil {
		return err
	}
	return nil
}

func (ud *urlDriver) Cache(u *urls.Url) error {
	for _, s := range badUrlStrings {
		if strings.Index(u.Url, s) != -1 {
			return fmt.Errorf("Url contains bad substring '%s'.", s)
		}
	}
	res, err := http.Get(u.Url)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("Received non-200 response '%s' from server.",
			res.Status)
	}
	// 1 << 22 == 4MB
	if res.ContentLength > 1 << 22 {
		return fmt.Errorf("Response too large (%d MB) to cache safely.",
			res.ContentLength/1024/1024)
	}
	u.CachedAs = ud.Encode(u.Url)
	fh, err := os.OpenFile(util.JoinPath(*urlCacheDir, u.CachedAs),
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(0600))
	defer fh.Close()
	if err != nil {
		return err
	}
	if _, err := io.Copy(fh, res.Body); err != nil {
		return err
	}
	u.CacheTime = time.Now()
	u.MimeType = res.Header.Get("Content-Type")
	if _, err := ud.Upsert(bson.M{"url": u.Url}, u); err != nil {
		return err
	}
	return nil
}