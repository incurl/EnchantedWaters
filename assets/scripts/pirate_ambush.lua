-- pirate_ambush.lua
--
-- A sample event script of the game.
--
-- The script describes what happens during a pirate encounter. It runs in
-- a sandbox: it can only describe the event, it cannot touch the
-- simulation.
--
-- The available API:
--   log(text)          adds a line to the event log
--   damage(amount)     the damage that is applied to the hull
--   resource(amount)   the change of the food and water stores
--   reputation(amount) the change of the reputation
--   cargo(fraction)    the fraction of the cargo that is lost
--   random(max)        a random number in [0, max)

local encounter = event.name or "A pirate encounter"

-- The severity of the encounter grows with the day: the pirates become
-- bolder over time.
local severity = random(10)

if severity < 4 then
    -- The pirates are scared off before they can do any harm.
    log("A pirate ship hails and asks for a parley.")
    log("They leave without a fight, but they follow us for a while.")
    return {
        damage = 0,
        reputation = 2,
        message = "A pirate ship hails and asks for a parley. They leave without a fight.",
    }
end

-- A real fight: the hull takes damage and the cargo is plundered.
local hullDamage = 4 + severity
local plundered = 0.1 + severity * 0.02

log(encounter .. ": the pirates close in!")
log("They board the ship while the crew fights them off.")

return {
    damage = hullDamage,
    cargo = plundered,
    reputation = -1,
    message = "Pirates board the ship! The fight costs us dearly.",
}
