import os
import sys
import shutil
import requests
import re
from urllib.parse import urlparse, parse_qs

def get_playlist_info(url):
    """从API获取歌单信息"""
    api_url = "http://192.168.31.6:8081/songlist"
    params = {
        "detailed": "true",
        "format": "song"
    }
    data = {
        "url": url
    }
    
    try:
        response = requests.post(api_url, params=params, data=data)
        response.raise_for_status()
        return response.json()
    except requests.exceptions.RequestException as e:
        print(f"获取歌单信息失败: {e}")
        sys.exit(1)

def normalize_filename(name):
    """标准化文件名，移除不合法字符"""
    return re.sub(r'[\\/*?:"<>|]', "", name)

def find_matching_songs(songs, directory):
    """在目录中查找匹配的歌曲"""
    found_songs = {}
    missing_songs = []
    
    # 获取所有音频文件
    audio_files = []
    for root, _, files in os.walk(directory):
        for file in files:
            if file.lower().endswith(('.mp3', '.flac', '.wav', '.m4a', '.ogg')):
                audio_files.append(os.path.join(root, file))
    
    # 为每首歌曲查找匹配文件
    for song in songs:
        found = False
        normalized_song = song.lower()
        
        for audio_file in audio_files:
            filename = os.path.basename(audio_file)
            filename_without_ext = os.path.splitext(filename)[0].lower()
            
            # 检查歌曲名是否在文件名中
            if normalized_song in filename_without_ext:
                found_songs[song] = audio_file
                found = True
                break
        
        if not found:
            missing_songs.append(song)
    
    return found_songs, missing_songs

def main():
    # 获取当前目录
    current_dir = os.getcwd()
    
    # 获取歌单URL
    playlist_url = input("请输入歌单分享链接: ")
    
    # 获取歌单信息
    print("正在获取歌单信息...")
    playlist_data = get_playlist_info(playlist_url)
    
    if playlist_data["code"] != 1:
        print(f"获取歌单失败: {playlist_data['msg']}")
        sys.exit(1)
    
    playlist_name = playlist_data["data"]["name"]
    songs = playlist_data["data"]["songs"]
    
    print(f"歌单名称: {playlist_name}")
    print(f"歌单中共有 {len(songs)} 首歌曲")
    
    # 询问用户要搜索的目录
    search_dir = input(f"请输入要搜索的目录 (直接回车使用当前目录 {current_dir}): ")
    if not search_dir:
        search_dir = current_dir
    
    if not os.path.exists(search_dir):
        print(f"目录 {search_dir} 不存在!")
        sys.exit(1)
    
    print(f"正在搜索目录 {search_dir} 中的歌曲...")
    
    # 查找匹配的歌曲
    found_songs, missing_songs = find_matching_songs(songs, search_dir)
    
    print(f"找到 {len(found_songs)} 首歌曲，缺少 {len(missing_songs)} 首歌曲")
    
    # 创建目标文件夹
    safe_playlist_name = normalize_filename(playlist_name)
    target_dir = os.path.join(current_dir, safe_playlist_name)
    
    if os.path.exists(target_dir):
        overwrite = input(f"目录 {target_dir} 已存在，是否覆盖? (y/n): ")
        if overwrite.lower() != 'y':
            target_dir = os.path.join(current_dir, f"{safe_playlist_name}_{os.urandom(2).hex()}")
    
    os.makedirs(target_dir, exist_ok=True)
    
    # 复制歌曲
    print(f"正在复制歌曲到 {target_dir}...")
    
    for song, source_path in found_songs.items():
        file_ext = os.path.splitext(source_path)[1]
        target_path = os.path.join(target_dir, normalize_filename(song) + file_ext)
        shutil.copy2(source_path, target_path)
    
    # 输出缺失的歌曲
    if missing_songs:
        missing_file = os.path.join(target_dir, "缺失歌曲.txt")
        with open(missing_file, "w", encoding="utf-8") as f:
            f.write(f"歌单 '{playlist_name}' 中缺失的歌曲：\n\n")
            for i, song in enumerate(missing_songs, 1):
                f.write(f"{i}. {song}\n")
        print(f"缺失的歌曲已保存到 {missing_file}")
    
    print(f"歌单 '{playlist_name}' 已成功创建！")

if __name__ == "__main__":
    main()
