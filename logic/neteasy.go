package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"GoMusic/misc/httputil"
	"GoMusic/misc/models"
	"GoMusic/misc/utils"
)

const (
	netEasyUrlV6 = "https://music.163.com/api/v6/playlist/detail"
	netEasyUrlV3 = "https://music.163.com/api/v3/song/detail"
	chunkSize    = 400
)

// NetEasyDiscover 获取网易云音乐歌单信息
// link: 歌单链接
// detailed: 是否使用详细歌曲名（原始歌曲名，不去除括号等内容）
func NetEasyDiscover(link string, detailed bool) (*models.SongList, error) {
	// 1. 获取歌单基本信息
	songIdsResp, err := getSongsInfo(link)
	if err != nil {
		return nil, fmt.Errorf("获取歌单信息失败: %w", err)
	}

	playlistName := songIdsResp.Playlist.Name      // 歌单名
	trackIds := songIdsResp.Playlist.TrackIds      // 歌曲ID列表
	tracksCount := songIdsResp.Playlist.TrackCount // 歌曲总数

	// 如果歌单为空，直接返回
	if len(trackIds) == 0 {
		return &models.SongList{
			Name:       playlistName,
			Songs:      []string{},
			SongsCount: 0,
		}, nil
	}

	// 收集所有歌曲ID
	allSongIds := make([]uint, len(trackIds))
	for i, track := range trackIds {
		allSongIds[i] = track.Id
	}

	// 存储歌曲信息的结果集
	resultMap := sync.Map{}

	// 直接从API获取所有歌曲信息
	allSongIdsSlice := make([]*models.SongId, len(allSongIds))
	for i, id := range allSongIds {
		allSongIdsSlice[i] = &models.SongId{Id: id}
	}

	// 分块处理，避免请求过大
	missSize := len(allSongIdsSlice)
	chunkCount := (missSize + chunkSize - 1) / chunkSize
	chunks := make([][]*models.SongId, chunkCount)

	for i := 0; i < missSize; i += chunkSize {
		end := i + chunkSize
		if end > missSize {
			end = missSize
		}
		chunks[i/chunkSize] = allSongIdsSlice[i:end]
	}

	// 并发请求处理
	var eg errgroup.Group

	for _, chunk := range chunks {
		chunk := chunk // 创建副本避免闭包问题
		eg.Go(func() error {
			return processChunkDetailed(chunk, &resultMap, detailed)
		})
	}

	// 等待所有请求完成
	if err := eg.Wait(); err != nil {
		return nil, fmt.Errorf("获取歌曲详情失败: %w", err)
	}

	// 返回最终结果
	return createSongList(playlistName, trackIds, resultMap, tracksCount), nil
}

// createSongList 创建歌单结果
func createSongList(name string, trackIds []*models.TrackId, resultMap sync.Map, count int) *models.SongList {
	return &models.SongList{
		Name:       name,
		Songs:      utils.SyncMapToSortedSlice(trackIds, resultMap),
		SongsCount: count,
	}
}

// getSongsInfo 获取歌单基本信息
func getSongsInfo(link string) (*models.NetEasySongId, error) {
	songListId, err := utils.GetNetEasyParam(link)
	if err != nil {
		return nil, fmt.Errorf("解析歌单链接失败: %w", err)
	}

	resp, err := httputil.Post(netEasyUrlV6, strings.NewReader("id="+songListId))
	if err != nil {
		return nil, fmt.Errorf("请求网易云API失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应内容失败: %w", err)
	}

	songIdsResp := &models.NetEasySongId{}
	if err = json.Unmarshal(body, songIdsResp); err != nil {
		return nil, fmt.Errorf("解析响应内容失败: %w", err)
	}

	if songIdsResp.Code == 401 {
		return nil, errors.New("无权限访问该歌单")
	}

	return songIdsResp, nil
}

// processChunkDetailed 处理一个分块的歌曲ID（详细模式）
func processChunkDetailed(chunk []*models.SongId, resultMap *sync.Map, detailed bool) error {
	// 1. 序列化请求参数
	marshal, err := json.Marshal(chunk)
	if err != nil {
		return fmt.Errorf("序列化请求参数失败: %w", err)
	}

	// 2. 发送请求
	resp, err := httputil.Post(netEasyUrlV3, strings.NewReader("c="+string(marshal)))
	if err != nil {
		return fmt.Errorf("请求歌曲详情失败: %w", err)
	}
	defer resp.Body.Close()

	// 3. 读取响应内容
	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应内容失败: %w", err)
	}

	// 4. 解析响应内容
	songs := &models.Songs{}
	if err = json.Unmarshal(bytes, songs); err != nil {
		return fmt.Errorf("解析响应内容失败: %w", err)
	}

	// 5. 处理歌曲信息
	for _, song := range songs.Songs {
		// 根据detailed参数决定是否使用原始歌曲名
		var songName string
		if detailed {
			songName = song.Name // 使用原始歌曲名
		} else {
			songName = utils.StandardSongName(song.Name) // 使用标准化的歌曲名
		}

		// 构建作者信息
		authors := make([]string, len(song.Ar))
		for i, ar := range song.Ar {
			authors[i] = ar.Name
		}

		// 格式化歌曲信息
		songInfo := fmt.Sprintf("%s - %s", songName, strings.Join(authors, " / "))

		// 存储结果
		resultMap.Store(song.Id, songInfo)
	}

	return nil
}
