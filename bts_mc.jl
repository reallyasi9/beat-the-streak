using ProgressMeter

in_file = ARGS[1]
week = 1
if (length(ARGS) >= 2)
  week = int(ARGS[2])
end

data = readcsv(in_file, Any, header=true, comment_char='%')

teams = data[1][:,1]
nteams = length(teams)

probs = data[1][:,2:end]
nweeks = size(probs, 2)

# FIXME This will accept any number of used teams, I just want up to week+1 (for dd)
used_teams = UTF8String[]
used_dd_team = ""
used_dd_week = 0
if week > 1
  for i in 3:length(ARGS)
    if isa(parse(ARGS[i]), Int)
      used_dd_week = int(ARGS[i])
      used_dd_team = ARGS[i+1]
      break
    else
      push!(used_teams, ARGS[i])
    end
  end
end

used_teams = indexin(used_teams, teams)
if used_dd_team != ""
  used_dd_team = indexin([used_dd_team], teams)
  used_dd_team = used_dd_team[1]
end
team_indices = collect(1:nteams)
these_used_teams = copy(used_teams)
if used_dd_week > 0
  push!(these_used_teams, used_dd_team)
end
deleteat!(team_indices, these_used_teams)

dd_already_happened = used_dd_week > 0

conv_probs = similar(probs, Float32)
for x in 1:size(probs,1), y in 1:size(probs,2)
  conv_probs[x,y] = probs[x,y] == "#N/A" ? NaN : probs[x,y]
end

@everywhere function fill_probabilities!(plist::Array{Float32}, teams::Array{Int}, probs::Array{Float32}, week::Int64)
  for i in 1:length(teams)
    @inbounds p = probs[teams[i], i + week - 1]
    if isnan(p)
      empty!(plist)
      break
    end
    push!(plist, p)
  end
end

@everywhere function find_dd(probs::Array{Float32}, teams::Array{Int64}, combined_teams::Array{Int64})
  dd_week = findfirst(teams, combined_teams[1])
  dd_prob = probs[combined_teams[2], dd_week]
  return (dd_prob, dd_week)
end

# List inversion: p[i]=j => p[j]=i
@everywhere function invert(p::Array{Int})
  # This fails majorly if it is compiled
  #return setindex!(p, 1:length(p), p)
  q = similar(p)
  for i = 1:length(p)
    q[p[i]] = i
  end
  return q
end

# Permute function
@everywhere function permute_teams(i::Int64,
                                   perms,
                                   probs::Array{Float32},
                                   week::Int64,
                                   combined_teams::Array{Int64},
                                   used_teams::Array{Int64})
  j = 0
  best_prob = 0.
  @showprogress 10 "Permuting..." for streak_team_order in perms
    j += 1
    plist = Float32[]
    fill_probabilities!(plist, streak_team_order, probs, week)
    if length(plist) > 0

      (dd_prob, dd_week) = find_dd(conv_probs, streak_team_order, combined_teams)
      push!(plist, dd_prob)

      plist = cumprod(plist)

      if plist[end] > best_prob
        best_prob = plist[end]

        # push in the missing teams so the inversion works
        for iteam in length(used_teams):-1:1
          unshift!(streak_team_order, used_teams[iteam])
        end
        push!(streak_team_order, combined_teams[2])
        streak_team_order = invert(streak_team_order)
        # now replace the week number of the dd team with its companion
        streak_team_order[combined_teams[2]] = streak_team_order[combined_teams[1]]

        out_stream = open("week$(week)_streak_output_$i.csv", "a")
        writedlm(out_stream, [time() i j transpose(streak_team_order) best_prob], ',')
        close(out_stream)
      end

    end
  end
end

@everywhere function permute_teams(perms,
                                   probs::Array{Float32},
                                   week::Int64,
                                   used_teams::Array{Int64},
                                   dd_team::Int64,
                                   dd_week::Int64)
  j = 0
  best_prob = 0.
  @showprogress 10 "Permuting..." for streak_team_order in perms
    j += 1
    plist = Float32[]
    fill_probabilities!(plist, streak_team_order, probs, week)
    if length(plist) > 0

      plist = cumprod(plist)

      if plist[end] > best_prob
        best_prob = plist[end]

        # push in the missing teams so the inversion works
        for iteam in length(used_teams):-1:1
          unshift!(streak_team_order, used_teams[iteam])
        end
        push!(streak_team_order, used_dd_team)
        streak_team_order = invert(streak_team_order)
        # now replace the week number of the dd team with its companion
        other_dd_team = findfirst(streak_team_order, dd_week)
        streak_team_order[dd_team] = streak_team_order[other_dd_team]

        out_stream = open("week$(week)_streak_output.csv", "a")
        writedlm(out_stream, [time() 1 j transpose(streak_team_order) best_prob], ',')
        close(out_stream)
      end

    end
  end
end

# If there were no bye weeks, there would be a total of nCr(nteams, 2) * nPr(nteams-1, nweeks) streaks.
# Note that nteams-1 == nweeks.
if !dd_already_happened
  combs = combinations(team_indices, 2) # does nCr(nteams, 2) when iterated over
  vcombs = collect(combs)
  @sync @parallel for i in 1:length(vcombs)

    # Begin the output
    out_stream = open("week$(week)_streak_output_$i.csv", "w")
    writedlm(out_stream, ["time" "combination" "permutation" transpose(teams) "probability"], ',')
    close(out_stream)

    combined_teams = vcombs[i]
    remaining_teams = team_indices
    splice!(remaining_teams, combined_teams[end]) # remove the second of the teams that share a week

    println("Combination ", i, ": ", remaining_teams, " + ", combined_teams)

    perms = permutations(remaining_teams)

    permute_teams(i, perms, conv_probs, week, combined_teams, used_teams)

  end

else

  # Begin the output
  out_stream = open("week$(week)_streak_output.csv", "w")
  writedlm(out_stream, ["time" "combination" "permutation" transpose(teams) "probability"], ',')
  close(out_stream)

  println("Combination: ", team_indices)

  perms = permutations(team_indices)

  permute_teams(perms, conv_probs, week, used_teams, used_dd_team, used_dd_week)

end

