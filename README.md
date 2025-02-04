# PerfectOddsBot

PerfectOddsBot is a Discord bot designed to create and manage bets with imaginary points. The bot allows users to place bets & view leaderboards. Admins can create and manage bets and bot settings

## Features

- Create and manage bets with multiple options.
- Users can place bets on available options.
- Admins can lock bets to prevent further betting.
- Admins can resolve bets and distribute points based on the outcome.
- Users can view their points and the leaderboard.
- Users gain points for sending messages

## Commands

### Slash Commands

| Command                   | Description                                                                                           | Admin Only | Premium | Ephemeral |
|---------------------------|-------------------------------------------------------------------------------------------------------|------------|---------|-----------|
| `/create-bet`             | Create a new bet with specified options and odds.                                                     | Yes        | No      | No        |
| `/create-cfb-bet`         | Create new CFB bet for provided game id                                                               | No         | Yes     | No        |
| `/create-cbb-bet`         | Create new CBB bet for provided game id                                                               | No         | Yes     | No        |
| `/resolve-bet`            | Resolve an existing bet by specifying the winning option.                                             | Yes        | No      | No        |
| `/give-points`            | Give points to a specific user.                                                                       | Yes        | No      | No        |
| `/reset-points`           | Reset all users' points to 1000.                                                                      | Yes        | No      | No        |
| `/leaderboard`            | Display the leaderboard with the top users based on points.                                           | No         | No      | No        |
| `/my-points`              | Display your current point total                                                                      | No         | No      | Yes       |
| `/my-bets`                | Display your active bets not yet resolved                                                             | No         | No      | Yes       |
| `/list-cfb-games`         | List all CFB games for current week and their spreads                                                 | No         | Yes     | Yes       |
| `/list-cbb-games`         | List all CBB games available and their spreads                                                        | No         | Yes     | Yes       |
| `/set-betting-channel`    | Set the current channel to your Server's 'bet channel' where auto msgs get sent                       | Yes        | No      | Yes       |
| `/set-points-per-message` | Set the amount of points a user will receive for each message they send                               | Yes        | No      | Yes       |
| `/set-starting-points`    | Set the amount of points a new user will start with                                                   | Yes        | No      | Yes       |
| `/subscribe-to-team`      | Select a team to subscribe to all events for. Bets will be auto-created for all their CBB & CFB games | Yes        | Yes     | Yes       |

### Interactions (Buttons)

- **Placing Bets:** Users can place bets by clicking the corresponding button on a bet message.
- **Lock Bet:** Admins can lock a bet to prevent further betting.
- **Resolve Bet:** Admins can resolve a bet to determine the winning option and distribute points accordingly.

### Schedule
- **Every day at 9am EST**: CFB Lines checked and updated
- **Every 5 minutes**: CFB & CBB Bets checked for game started to lock the bet
- **Every hour**: CFB & CBB Bets checked for game ended to payout bet

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
