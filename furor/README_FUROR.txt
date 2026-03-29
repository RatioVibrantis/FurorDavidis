FUROR DAVIDIS — AI Engine Setup
================================

Шаг 1: Скачать llamafile runtime
  https://github.com/Mozilla-Ocho/llamafile/releases
  → скачать llamafile-server-*.exe или llamafile-*.exe

Шаг 2: Скачать модель Qwen3-1.7B-Q4_K_M (~1.1GB)
  https://huggingface.co/Qwen/Qwen3-1.7B-GGUF
  → файл: qwen3-1.7b-instruct-q4_k_m.gguf

  Альтернатива (меньше, быстрее, менее умная):
  https://huggingface.co/Qwen/Qwen3-0.6B-GGUF
  → файл: qwen3-0.6b-instruct-q4_k_m.gguf

Шаг 3: Объединить в один llamafile (опционально)
  llamafile-server.exe --model qwen3-1.7b-instruct-q4_k_m.gguf --unsecure --host 127.0.0.1

Шаг 4: Положить furor.exe рядом с FurorDavidis.exe
  FurorDavidis.exe
  furor\
      furor.exe     ← llamafile

  Или указать путь вручную в Settings → AI Модель

Требования:
  RAM: ~2GB свободно (для 1.7B модели)
  CPU: любой x86-64 (GPU ускорение если есть CUDA/ROCm — автоматически)
