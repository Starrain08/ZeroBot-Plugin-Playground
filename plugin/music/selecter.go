// Package music QQ音乐、网易云、酷狗、酷我、咪咕 点歌
package music

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/FloatTech/floatbox/web"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
	"github.com/tidwall/gjson"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

const (
	defaultCoverURL = "https://p1.music.126.net/VnZiScyynLG7at3ITfA2ig==/default_cover.jpg"
)

type MusicCard struct {
	Type      string
	Title     string
	Artist    string
	AudioURL  string
	SourceURL string
	ImageURL  string
}

func init() {
	control.AutoRegister(&ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Brief:            "点歌",
		Help: "- 点歌[xxx]\n" +
			"- 网易点歌[xxx]\n" +
			"- 酷我点歌[xxx]\n" +
			"- 酷狗点歌[xxx]\n" +
			"- 咪咕点歌[xxx]",
	}).OnRegex(`^(.{0,2})点歌\s?(.{1,25})$`).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(func(ctx *zero.Ctx) {
			platform := ctx.State["regex_matched"].([]string)[1]
			keyword := ctx.State["regex_matched"].([]string)[2]

			var (
			        card MusicCard
			        err error
			)
			switch platform {
			case "咪咕":
				card = migu(keyword)
			case "酷我":
				card = kuwo(keyword)
			case "酷狗":
				card = kugou(keyword)
			case "网易":
				card = cloud163(keyword)
			default:
				// 唯一需要处理error的分支
				card, err = qqmusic(keyword)
				if err != nil {
					ctx.SendChain(message.Text("QQ音乐点歌失败: ", err.Error()))
					return
				}
			}

			if card.Title == "" || card.AudioURL == "" {
				ctx.SendChain(message.Text("未找到相关歌曲"))
				return
			}

			ctx.SendChain(buildMusicCard(card))
		})
}

func buildMusicCard(card MusicCard) message.Segment {
	if !isValidImageURL(card.ImageURL) {
		card.ImageURL = defaultCoverURL
	}

	return message.CustomMusic(
		card.SourceURL,
		card.AudioURL,
		card.Title,
	).Add("content", card.Artist).
		Add("image", card.ImageURL).
		Add("type", card.Type)
}

// 优化封面有效性检查
func isValidImageURL(url string) bool {
    return strings.HasPrefix(url, "http") && 
        (strings.Contains(url, ".jpg") || 
         strings.Contains(url, ".jpeg") || 
         strings.Contains(url, ".png") || 
         strings.Contains(url, ".webp"))
}

// 咪咕音乐（新接口版本）
func migu(keyword string) MusicCard {
	apiURL := fmt.Sprintf(
		"https://www.hhlqilongzhu.cn/api/dg_mgmusic.php?n=1&type=json&gm=%s",
		url.QueryEscape(keyword),
	)
	data, err := web.GetData(apiURL)
	if err != nil {
		return MusicCard{}
	}

	result := gjson.ParseBytes(data)
	if result.Get("code").Int() != 200 || !result.Exists() {
		return MusicCard{}
	}

	// 验证关键字段
	audioURL := result.Get("music_url").Str
	if audioURL == "" || !strings.HasPrefix(audioURL, "http") {
		return MusicCard{}
	}

	// 处理封面
	imageURL := result.Get("cover").Str
	if !isValidImageURL(imageURL) {
		imageURL = defaultCoverURL
	}

	return MusicCard{
		Type:      "migu",
		Title:     result.Get("title").Str,
		Artist:    result.Get("singer").Str,
		AudioURL:  audioURL,
		SourceURL: result.Get("link").Str,
		ImageURL:  imageURL,
	}
}

// 酷我音乐（新接口实现）
func kuwo(keyword string) MusicCard {
    apiURL := fmt.Sprintf(
        "https://www.hhlqilongzhu.cn/api/dg_kuwomusic.php?n=1&type=json&msg=%s",
        url.QueryEscape(keyword),
    )
    data, err := web.GetData(apiURL)
    if err != nil {
        return MusicCard{}
    }

    result := gjson.ParseBytes(data)
    
    // 处理字符串类型的code字段
    if result.Get("code").String() != "200" || !result.Exists() {
        return MusicCard{}
    }

    audioURL := result.Get("flac_url").Str
    if audioURL == "" || !strings.HasPrefix(audioURL, "http") {
        return MusicCard{}
    }

    imageURL := result.Get("cover").Str
    if !isValidImageURL(imageURL) {
        imageURL = defaultCoverURL
    }

    return MusicCard{
        Type:      "kuwo",
        Title:     result.Get("song_name").Str,
        Artist:    result.Get("song_singer").Str,
        AudioURL:  audioURL,
        SourceURL: result.Get("link").Str,
        ImageURL:  imageURL,
    }
}

// 酷狗音乐（新版接口实现）
func kugou(keyword string) MusicCard {
	apiURL := fmt.Sprintf(
		"https://www.hhlqilongzhu.cn/api/dg_kugouSQ.php?n=1&type=json&msg=%s",
		url.QueryEscape(keyword),
	)
	data, err := web.GetData(apiURL)
	if err != nil {
		return MusicCard{} // 网络请求失败
	}

	result := gjson.ParseBytes(data)
	if result.Get("code").Int() != 200 || !result.Exists() {
		return MusicCard{} // 接口异常或数据无效
	}

	// 验证关键字段
	audioURL := result.Get("music_url").Str
	if audioURL == "" || !strings.HasPrefix(audioURL, "http") {
		return MusicCard{} // 音频链接无效
	}

	// 处理封面有效性
	imageURL := result.Get("cover").Str
	if !isValidImageURL(imageURL) {
		imageURL = defaultCoverURL
	}

	return MusicCard{
		Type:      "kugou",
		Title:     result.Get("title").Str,
		Artist:    result.Get("singer").Str,
		AudioURL:  audioURL,
		SourceURL: result.Get("link").Str,
		ImageURL:  imageURL,
	}
}

// 网易云音乐（新接口版本）
func cloud163(keyword string) MusicCard {
	apiURL := fmt.Sprintf(
		"https://www.hhlqilongzhu.cn/api/dg_wyymusic.php?type=json&n=1&num=1&gm=%s",
		url.QueryEscape(keyword),
	)
	data, err := web.GetData(apiURL)
	if err != nil {
		return MusicCard{} // 网络请求失败
	}

	result := gjson.ParseBytes(data)
	if result.Get("code").Int() != 200 || !result.Exists() {
		return MusicCard{} // 接口异常
	}

	// 处理封面有效性
	imageURL := result.Get("cover").Str
	if !isValidImageURL(imageURL) {
		imageURL = defaultCoverURL
	}

	return MusicCard{
		Type:      "163",
		Title:     result.Get("title").Str,
		Artist:    result.Get("singer").Str,
		AudioURL:  result.Get("music_url").Str,
		SourceURL: result.Get("link").Str,
		ImageURL:  imageURL,
	}
}

// QQ音乐（修复封面逻辑）
func qqmusic(keyword string) (MusicCard, error) {
    // Step 1: 获取歌曲MID
    searchURL := fmt.Sprintf(
        "https://c.y.qq.com/splcloud/fcgi-bin/smartbox_new.fcg?platform=yqq.json&key=%s",
        url.QueryEscape(keyword),
    )
    searchData, err := web.RequestDataWith(
        web.NewDefaultClient(),
        searchURL,
        "GET", "", web.RandUA(), nil,
    )
    if err != nil {
        return MusicCard{}, fmt.Errorf("请求失败: %v", err)
    }

    songInfo := gjson.ParseBytes(searchData).Get("data.song.itemlist.0")
    if !songInfo.Exists() {
        return MusicCard{}, fmt.Errorf("未找到相关歌曲")
    }

    mid := songInfo.Get("mid").Str
    if mid == "" {
        return MusicCard{}, fmt.Errorf("歌曲MID缺失")
    }

    // Step 2: 获取封面（优化逻辑）
    var coverURL string
    coverAPI := fmt.Sprintf(
        "https://www.hhlqilongzhu.cn/api/dg_shenmiMusic_SQ.php?n=1&type=json&msg=%s",
        url.QueryEscape(keyword),
    )
    if coverData, err := web.GetData(coverAPI); err == nil {
        result := gjson.ParseBytes(coverData)
        if result.Get("code").Int() == 200 {
            if tmpURL := result.Get("data.cover").Str; tmpURL != "" {
                if isValidImageURL(tmpURL) {
                    coverURL = tmpURL
                } else {
                    coverURL = defaultCoverURL // 显式回退默认
                }
            }
        }
    }

    // 如果未获取到有效封面，使用默认
    if coverURL == "" {
        coverURL = defaultCoverURL
    }

    return MusicCard{
        Type:      "qq",
        Title:     songInfo.Get("name").Str,
        Artist:    songInfo.Get("singer").Str,
        AudioURL:  fmt.Sprintf("https://dl.stream.qqmusic.qq.com/C400%s.m4a", mid),
        SourceURL: fmt.Sprintf("https://y.qq.com/n/ryqq/songDetail/%s", mid),
        ImageURL:  coverURL,
    }, nil
}

func md5str(s string) string {
    h := md5.New()
    h.Write([]byte(s))
    return strings.ToUpper(hex.EncodeToString(h.Sum(nil)))
}

func netGet(url string, header http.Header) []byte {
    client := &http.Client{Timeout: 10 * time.Second}
    req, _ := http.NewRequest("GET", url, nil)
    if header != nil {
        req.Header = header
    }
    res, err := client.Do(req)
    if err != nil {
        return nil
    }
    defer res.Body.Close()
    data, _ := io.ReadAll(res.Body)
    return data
}