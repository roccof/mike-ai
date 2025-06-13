# Mike AI

An AI painter that creates artwork through voice conversation.

## What it does

Mike is an AI that you can talk to, and he'll paint pictures on a canvas in real-time. Just say "Mike, paint a house" and watch him draw it.

## How to run it

1. **Build and run a Docker container**:
   ```bash
   docker build -t mike-ai .
   docker run -p 8080:8080 -e OPENAI_API_KEY="your-key" mike-ai
   ```

2. **Open your browser** to http://127.0.0.1:8080
