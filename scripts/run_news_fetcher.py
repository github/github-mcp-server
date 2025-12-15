import os
from fetch_eodhd_news import fetch_news, display_news

# The API key provided by the user.
API_TOKEN = "690d7cdc3013f4.57364117"
SYMBOLS = "AAPL.US,SPY.US"
LIMIT = 5

def main():
    print(f"Executing news fetch for symbols: {SYMBOLS}")
    news_items = fetch_news(api_token=API_TOKEN, symbols=SYMBOLS, limit=LIMIT)
    if news_items:
        display_news(news_items)
    else:
        print("Did not receive any news items.")

if __name__ == "__main__":
    main()
