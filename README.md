# PerfectOddsBot

PerfectOddsBot is a Discord bot designed to manage and interact with betting odds within a Discord server. The bot allows users to place bets, view leaderboards, and admins can lock and resolve bets.

## Features

- Create and manage bets with multiple options.
- Users can place bets on available options.
- Admins can lock bets to prevent further betting.
- Admins can resolve bets and distribute points based on the outcome.
- Users can view their points and the leaderboard.
- Users gain points for sending messages

## Commands

### Slash Commands

| Command               | Description                                                                 | Admin Only |
|-----------------------|-----------------------------------------------------------------------------|------------|
| `/createbet`          | Create a new bet with specified options and odds.                           | Yes        |
| `/resolvebet`         | Resolve an existing bet by specifying the winning option.                   | Yes        |
| `/givepoints`         | Give points to a specific user.                                             | Yes        |
| `/resetpoints`        | Reset all users' points to 1000.                                            | Yes        |
| `/points`             | Display your current points.                                                | No         |
| `/leaderboard`        | Display the leaderboard with the top users based on points.                 | No         |

### Interactions (Buttons)

- **Placing Bets:** Users can place bets by clicking the corresponding button on a bet message.
- **Lock Bet:** Admins can lock a bet to prevent further betting.
- **Resolve Bet:** Admins can resolve a bet to determine the winning option and distribute points accordingly.

## Privacy Information

### Data Collected

PerfectOddsBot collects and stores the following data:

- **User IDs and Guild IDs:** To track points and bets tied to specific users and Discord servers.
- **Bets and Bet Entries:** Information about the bets created and the entries (bets placed by users).

### Data Usage

The data collected by PerfectOddsBot is used solely for the purpose of providing betting functionalities within Discord. No data is shared with third parties. Only Discord ID and Server ID are stored.

### Data Retention

Data is retained as long as the bot is active in a server. If the bot is removed, users can request the deletion of their data.

## How to Run the Bot

### Prerequisites

- **Go:** Make sure you have Go installed on your system. You can download it from [golang.org](https://golang.org/dl/).
- **MySQL:** A MySQL database is required to store the bot's data.

### Setup

1. **Clone the repository**

2. **Create a `.env` file:**
    - The `.env` file should contain your bot token and MySQL connection string.
    - Example `.env` file:
      ```
      DISCORD_BOT_TOKEN=your_bot_token_here
      MYSQL_URL=your_mysql_connection_string_here
      ```

3. **Build and run the bot:**
    ```bash
    go build -o PerfectOddsBot
    ./PerfectOddsBot
    ```

4. **Invite the bot to your server:**
    - Generate an OAuth2 URL from the [Discord Developer Portal](https://discord.com/developers/applications) and invite the bot to your server with the necessary permissions.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for more details.
