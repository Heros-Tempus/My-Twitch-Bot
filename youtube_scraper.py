import csv
import sys
import yt_dlp

def scrape_youtube(url, output_csv="tracks.csv"):
    ydl_opts: yt_dlp._Params = {
        'extract_flat': False,
        'skip_download': True, # type: ignore
        'quiet': True,
        'no_warnings': True,
    }

    print(f"Fetching metadata from: {url} ...")
    
    try:
        with yt_dlp.YoutubeDL(ydl_opts) as ydl:
            info = ydl.extract_info(url, download=False)
    except Exception as e:
        print(f"Error extracting metadata: {e}")
        sys.exit(1)

    if 'entries' in info:
        entries = info['entries']
    else:
        entries = [info]

    fields = [
        'video_id', 
        'url', 
        'track', 
        'artist', 
        'album', 
        'source_media', 
        'original_composer', 
        'duration_seconds'
    ]

    row_count = 0
    with open(output_csv, mode='w', newline='', encoding='utf-8') as csv_file:
        writer = csv.DictWriter(csv_file, fieldnames=fields)
        writer.writeheader()

        for entry in entries:
            if not entry:
                continue

            video_id = entry.get('id')
            if not video_id:
                continue

            duration = entry.get('duration')
            if duration is None:
                print(f"Skipping video {video_id} ({entry.get('title', 'Unknown Title')}) - duration is missing.")
                continue

            track = entry.get('track') or entry.get('title') or "Unknown Track"
            artist = entry.get('artist') or entry.get('uploader') or entry.get('channel') or "Unknown Artist"
            album = entry.get('album') or ""
            
            source_media = entry.get('series') or ""
            original_composer = entry.get('composer') or ""
            
            video_url = f"https://www.youtube.com/watch?v={video_id}"

            writer.writerow({
                'video_id': video_id,
                'url': video_url,
                'track': track,
                'artist': artist,
                'album': album,
                'source_media': source_media,
                'original_composer': original_composer,
                'duration_seconds': round(duration)
            })
            row_count += 1

    print(f"Successfully scraped {row_count} tracks and saved them to '{output_csv}'.")

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python scrape_youtube.py <YouTube_URL>")
        sys.exit(1)

    target_url = sys.argv[1]
    scrape_youtube(target_url)