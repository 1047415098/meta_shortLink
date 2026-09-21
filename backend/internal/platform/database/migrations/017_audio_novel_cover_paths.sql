-- Cover files were copied to the renamed persistent volume; translate only the retired public prefix.
UPDATE audio_novels
SET cover_path = '/audio-novel-uploads/' || substr(cover_path, length('/novel-uploads/') + 1),
    updated_at = now()
WHERE cover_path LIKE '/novel-uploads/%';
