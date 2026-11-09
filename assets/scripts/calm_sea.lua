-- calm_sea.lua
--
-- The script of a peaceful day at sea. It shows how a script can
-- modulate an event without replacing it.

log("The sea is calm and the wind is gentle.")

-- A small chance of finding something floating on the water.
local find = random(100)
if find < 15 then
    log("A drifting crate is spotted and fished aboard.")
    return {
        resource = 3,
        message = "A calm day: a drifting crate is fished aboard.",
    }
end

return {
    damage = 0,
    message = "A calm day at sea.",
}
