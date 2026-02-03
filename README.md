# PerfectOddsBot

PerfectOddsBot is a Discord bot designed to create and manage bets with imaginary points. The bot allows users to place bets & view leaderboards. Admins can create and manage bets and bot settings

## Features

- Create and manage bets with multiple options.
- Users can place bets on available options.
- Admins can lock bets to prevent further betting.
- Admins can resolve bets and distribute points based on the outcome.
- Users can view their points and the leaderboard.
- Users gain points for sending messages
- Card game layer: spend points to draw cards, build an inventory, and trigger effects that can impact points, bets, and other users.

## Card Game

PerfectOddsBot includes an optional “card game” that sits on top of the points + betting system.

### How drawing cards works

- **Command**: Use `/draw-card` to draw a random card from the deck.
- **Cost & pool**: Drawing costs points and (by default) the same amount is added to your server’s **Pool**. Some cards can also add/remove points from the pool. Certain Mythic cards pay out from the pool.
- **Cooldown cycle + escalating price**: Each user has a draw “cycle” that resets every (default: 60 minutes).
  - 1st draw in the cycle costs `CardDrawCost` (default: 10 points)
  - 2nd draw costs `CardDrawCost * 10`
  - 3rd+ draws cost `CardDrawCost * 50`
- **Server toggle**: Admins can enable/disable drawing per server with `/toggle-card-drawing`.
- **Rarities**: Cards come in **Common**, **Uncommon**, **Rare**, **Epic**, and **Mythic** rarities, with Mythic being the rarest.
- **Subscription-gated cards**: Some cards are only eligible if your server has a subscribed team set via `/subscribe-to-team` (these are treated as “premium” cards in the deck logic).

### Instant cards vs inventory cards

When you draw a card, it either:

- **Resolves immediately**: the handler runs right away (e.g. gain/lose points, steal points, global effects).
- **Goes into your inventory**: some cards are stored in `UserInventory` so they can be consumed later.
  - Use `/my-inventory` to see what you’re holding.
  - Some inventory cards are **automatic** and trigger on the next applicable event (usually bet resolution), then get consumed.
  - Some inventory cards are **playable** (manual). Currently, `/play-card` supports **STOP THE STEAL** (cancel an active bet you’ve placed).

### Follow-up selections (targets, bets, choices)

Some cards require extra input after you draw them:

- **Target a user**: you’ll be prompted to pick a user (examples: Petty Theft, The Jester, Bet Freeze, Grand Larceny, Anti-Anti-Bet, Hostile Takeover).
- **Target a bet**: you’ll be prompted to pick an active bet you placed (example: Uno Reverse).
- **Choose an option**: choice cards will prompt a selection (example: The Gambler).

### Scheduled card maintenance

Some card effects are handled by scheduled jobs:

- **Every hour**: collect **Loan Shark** debts after they mature, and expire **Vampire** cards after 24 hours.

## Commands

### Slash Commands

| Command                   | Description                                                                                           | Admin Only | Premium | Ephemeral |
|---------------------------|-------------------------------------------------------------------------------------------------------|------------|---------|-----------|
| `/help`                   | Show this help message with all available commands                                                    | No         | No      | Yes       |
| `/create-bet`             | Create a new bet with specified options and odds.                                                     | Yes        | No      | No        |
| `/create-cfb-bet`         | Create new CFB bet for provided game id                                                               | No         | Yes     | No        |
| `/create-cbb-bet`         | Create new CBB bet for provided game id                                                               | No         | Yes     | No        |
| `/create-parlay`          | Create a parlay by combining multiple open bets                                                        | No         | No      | No        |
| `/give-points`            | Give points to a specific user.                                                                       | Yes        | No      | No        |
| `/reset-points`           | Reset all users' points to a default value                                                            | Yes        | No      | No        |
| `/leaderboard`            | Display the leaderboard with the top users based on points.                                           | No         | No      | No        |
| `/my-points`              | Display your current point total                                                                      | No         | No      | Yes       |
| `/my-stats`               | Show your betting statistics                                                                          | No         | No      | Yes       |
| `/my-bets`                | Display your active bets not yet resolved                                                             | No         | No      | Yes       |
| `/my-parlays`             | Show your active parlays                                                                              | No         | No      | Yes       |
| `/draw-card`              | Draw a random card from the deck (cost increases per draw cycle; adds to pool)                        | No         | No      | No        |
| `/my-inventory`           | View the cards currently in your hand                                                                 | No         | No      | Yes       |
| `/play-card`              | Play a card from your inventory                                   | No         | No      | Yes       |
| `/list-cfb-games`         | List this weeks CFB games and their current lines                                                     | No         | Yes     | Yes       |
| `/list-cbb-games`         | List the currently open CBB games                                                                     | No         | Yes     | Yes       |
| `/set-betting-channel`    | Set the current channel to your Server's 'bet channel' where auto msgs get sent                       | Yes        | No      | Yes       |
| `/set-points-per-message` | Set the amount of points a user will receive for each message they send                               | Yes        | No      | Yes       |
| `/set-starting-points`    | Set the amount of points a new user will start with                                                   | Yes        | No      | Yes       |
| `/subscribe-to-team`      | Choose a College team to subscribe to all CFB & CBB events for                                        | Yes        | Yes     | Yes       |
| `/toggle-card-drawing`    | Toggle card drawing on/off for this server                                                            | Yes        | No      | Yes       |

### Interactions (Buttons)

- **Placing Bets:** Users can place bets by clicking the corresponding button on a bet message.
- **Lock Bet:** Admins can lock a bet to prevent further betting.
- **Resolve Bet:** Admins can resolve a bet to determine the winning option and distribute points accordingly.

### Schedule
- **Every day at 9am EST**: CFB Lines checked and updated
- **Every 5 minutes**: CFB & CBB Bets checked for game started to lock the bet
- **Every hour**: CFB & CBB Bets checked for game ended to payout bet
- **Every hour**: Card maintenance (Loan Shark collections, Vampire expirations)

## Privacy Information

### Data Collected

PerfectOddsBot collects and stores the following data:

- **User IDs and Guild IDs:** To track points and bets tied to specific users and Discord servers.
- **Bets and Bet Entries:** Information about the bets created and the entries (bets placed by users).
- **Card game state:** Server pool balance, per-user draw cooldown state, and per-user card inventory (including any card targets like a bet or user).

### Data Usage

The data collected by PerfectOddsBot is used solely for the purpose of providing betting functionalities within Discord. No data is shared with third parties. Only Discord ID and Server ID are stored.

### Data Retention

Data is retained as long as the bot is active in a server. If the bot is removed, users can request the deletion of their data.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for more details.
