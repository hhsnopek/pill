# :pill:
> Red or Blue?

## Features
- Minimal Configuartion
- Slack Notifications

## Installation

## Configuration
```
// pill.json
{
  "slack": {
    "webhook": "", // required
    "channel": "" // required
  },
  "cron-expression": "@every 5s", // required
  "sites": [
    {
      "url": "https://youtube.com", // required
      "channel": "#youtube", // optional
      "cron-expressions"": "@every 1s" // optional
    }
  ]
}
```

## Usage
`pill`

## License
MIT © [Henry Snopek](https://hhsnopek.com)
