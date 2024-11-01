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

| Command                | Description                                                                     | Admin Only | Ephemeral |
|------------------------|---------------------------------------------------------------------------------|------------|-----------|
| `/create-bet`          | Create a new bet with specified options and odds.                               | Yes        | No        |
| `/create-cfb-bet`      | Create new CFB bet for provided game id                                         | No         | No        |
| `/resolve-bet`         | Resolve an existing bet by specifying the winning option.                       | Yes        | No        |
| `/give-points`         | Give points to a specific user.                                                 | Yes        | No        |
| `/reset-points`        | Reset all users' points to 1000.                                                | Yes        | No        |
| `/leaderboard`         | Display the leaderboard with the top users based on points.                     | No         | No        |
| `/my-points`           | Display your current point total                                                | No         | Yes       |
| `/my-bets`             | Display your active bets not yet resolved                                       | No         | Yes       |
| `/list-cfb-games`      | List all CFB games for current week and their spreads                           | No         | Yes       |
| `/set-betting-channel` | Set the current channel to your Server's 'bet channel' where auto msgs get sent | Yes        | Yes       |

### Interactions (Buttons)

- **Placing Bets:** Users can place bets by clicking the corresponding button on a bet message.
- **Lock Bet:** Admins can lock a bet to prevent further betting.
- **Resolve Bet:** Admins can resolve a bet to determine the winning option and distribute points accordingly.

### Schedule
- **Every day at 9am EST**: CFB Lines checked and updated
- **Every 5 minutes**: CFB Bets checked for game started to lock the bet
- **Every hour**: CFB Bets checked for game ended to payout bet

## Privacy Information

### Data Collected

PerfectOddsBot collects and stores the following data:

- **User IDs and Guild IDs:** To track points and bets tied to specific users and Discord servers.
- **Bets and Bet Entries:** Information about the bets created and the entries (bets placed by users).

### Data Usage

The data collected by PerfectOddsBot is used solely for the purpose of providing betting functionalities within Discord. No data is shared with third parties. Only Discord ID and Server ID are stored.

### Data Retention

Data is retained as long as the bot is active in a server. If the bot is removed, users can request the deletion of their data.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for more details.
