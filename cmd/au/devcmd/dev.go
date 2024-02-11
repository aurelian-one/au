package devcmd

import (
	"context"
	"encoding/base64"
	"fmt"
	"hash/crc32"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/automerge/automerge-go"
	"github.com/oklog/ulid/v2"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/aurelian-one/au/cmd/au/common"
	"github.com/aurelian-one/au/pkg/au"
)

var Command = &cobra.Command{
	Use:   "dev",
	Short: "Render advanced development views of the active Workspace",
}

var dumpCommand = &cobra.Command{
	Use:  "dump",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		s := cmd.Context().Value(common.StorageContextKey).(au.StorageProvider)
		w := cmd.Context().Value(common.CurrentWorkspaceIdContextKey).(string)
		if w == "" {
			return errors.New("current workspace not set")
		}
		ws, err := s.OpenWorkspace(cmd.Context(), w, false)
		if err != nil {
			return err
		}
		defer ws.Close()

		dws, ok := ws.(au.DocProvider)
		if !ok {
			return errors.New("no access to doc")
		}
		doc := dws.GetDoc()

		encoder := yaml.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent(2)
		return encoder.Encode(toTree(doc.Root()))
	},
}

var generateYamlHistory = &cobra.Command{
	Use:  "history",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		s := cmd.Context().Value(common.StorageContextKey).(au.StorageProvider)
		w := cmd.Context().Value(common.CurrentWorkspaceIdContextKey).(string)
		if w == "" {
			return errors.New("current workspace not set")
		}
		ws, err := s.OpenWorkspace(cmd.Context(), w, false)
		if err != nil {
			return err
		}
		defer ws.Close()

		dws, ok := ws.(au.DocProvider)
		if !ok {
			return errors.New("no access to doc")
		}
		doc := dws.GetDoc()

		output := make([]map[string]interface{}, 0)
		changes, err := doc.Changes()
		if err != nil {
			return errors.Wrap(err, "failed to get changes")
		}
		for _, change := range changes {
			dependencies := make([]string, 0)
			for _, hash := range change.Dependencies() {
				dependencies = append(dependencies, hash.String())
			}
			output = append(output, map[string]interface{}{
				"hash":         change.Hash().String(),
				"at":           change.Timestamp(),
				"message":      change.Message(),
				"dependencies": dependencies,
			})
		}
		encoder := yaml.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent(2)
		return encoder.Encode(output)
	},
}

var generateDotHistory = &cobra.Command{
	Use:  "history-dot",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		s := cmd.Context().Value(common.StorageContextKey).(au.StorageProvider)
		w := cmd.Context().Value(common.CurrentWorkspaceIdContextKey).(string)
		if w == "" {
			return errors.New("current workspace not set")
		}
		ws, err := s.OpenWorkspace(cmd.Context(), w, false)
		if err != nil {
			return err
		}
		defer ws.Close()

		dws, ok := ws.(au.DocProvider)
		if !ok {
			return errors.New("no access to doc")
		}
		doc := dws.GetDoc()

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "strict digraph {")
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "node [colorscheme=pastel19]")
		changes, err := doc.Changes()
		if err != nil {
			return errors.Wrap(err, "failed to get changes")
		}
		for _, change := range changes {
			var color int
			{
				h := crc32.NewIEEE()
				_, _ = h.Write([]byte(change.ActorID()))
				color = 1 + int(h.Sum32()%9)
			}
			_, _ = fmt.Fprintf(
				cmd.OutOrStdout(), "\"%s\" [label=\"%s %s: '%s'\", style=\"filled\" fillcolor=%d]\n",
				change.Hash().String(), change.Hash().String()[:8], change.Timestamp().Format(time.RFC3339), change.Message(), color,
			)
			for _, hash := range change.Dependencies() {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\"%s\" -> \"%s\"\n", hash.String(), change.Hash().String())
			}
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "}")
		return nil
	},
}

func toTree(item *automerge.Value) interface{} {
	switch item.Kind() {
	case automerge.KindMap:
		out := make(map[string]interface{}, item.Map().Len())
		keys, _ := item.Map().Keys()
		for _, k := range keys {
			x, _ := item.Map().Get(k)
			out[k] = toTree(x)
		}
		return out
	case automerge.KindList:
		out := make([]interface{}, item.List().Len())
		for i := range out {
			x, _ := item.List().Get(i)
			out[i] = toTree(x)
		}
		return out
	case automerge.KindStr:
		return item.Str()
	case automerge.KindBytes:
		if len(item.Bytes()) > 1023 {
			return base64.StdEncoding.EncodeToString(item.Bytes()[:1020]) + "..."
		}
		return base64.StdEncoding.EncodeToString(item.Bytes())
	case automerge.KindText:
		raw, _ := item.Text().Get()
		return raw
	case automerge.KindInt64:
		return item.Int64()
	case automerge.KindFloat64:
		return item.Float64()
	case automerge.KindBool:
		return item.Bool()
	case automerge.KindCounter:
		v, _ := item.Counter().Get()
		return v
	case automerge.KindNull:
		return nil
	case automerge.KindTime:
		return item.Time().Format(time.RFC3339)
	case automerge.KindUint64:
		return item.Uint64()
	default:
		return item.GoString()
	}
}

var generateFakeData = &cobra.Command{
	Use:   "fake-data",
	Short: "Clone the workspace and add fake data to it",
	Long: strings.TrimSpace(`
This command is used during development to generate fake data in the given workspace. This generates a series of --num changes with a random distribution between creating, editing, and deleting things. The workspace is cloned, so that it doesn't affect the original workspace and so that users do not use the command by mistake on real live workspaces!

This command generates a linear history. To generate a branching history, the workspace must be copied to another Aurelian directory where a diverging history can be produced before being synchronised back to the original document.
`),
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		s := cmd.Context().Value(common.StorageContextKey).(au.StorageProvider)
		w := cmd.Context().Value(common.CurrentWorkspaceIdContextKey).(string)
		if w == "" {
			return errors.New("current workspace not set")
		}

		numOperations, err := cmd.Flags().GetInt("num")
		if err != nil {
			return err
		} else if numOperations < 1 {
			return errors.New("num must be at least 1")
		}

		ws, err := s.OpenWorkspace(cmd.Context(), w, false)
		if err != nil {
			return err
		}

		cloneWs, err := s.ImportWorkspace(cmd.Context(), ulid.Make().String(), ws.(au.DocProvider).GetDoc().Save())
		if err != nil {
			return errors.Wrap(err, "failed to clone")
		}

		ws, err = s.OpenWorkspace(cmd.Context(), cloneWs.Id, true)
		if err != nil {
			return err
		}
		defer ws.Close()

		dws := ws.(au.DocProvider)
		{
			dws.GetDoc().Path("created_at").Set(time.Now().UTC())
			oldAliasValue, _ := dws.GetDoc().Path("alias").Get()
			dws.GetDoc().Path("alias").Set(oldAliasValue.Str() + "with fake data")
		}

		for numOperations > 0 {
			n, err := fakeDataGenerators.Next(cmd.Context(), ws)
			if err != nil {
				return err
			}
			numOperations -= n
		}

		if err := ws.Flush(); err != nil {
			return err
		}

		encoder := yaml.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent(2)
		return encoder.Encode(cloneWs.Id)
	},
}

func init() {
	generateFakeData.Flags().Int("num", 10, "The number of operations to perform")

	Command.AddCommand(
		dumpCommand,
		generateYamlHistory,
		generateDotHistory,
		generateFakeData,
	)
}

type WeightedPath struct {
	Weight float64
	Next   func(ctx context.Context, sp au.WorkspaceProvider) (int, error)
}

func NewWeightedPath(w float64, n func(ctx context.Context, sp au.WorkspaceProvider) (int, error)) WeightedPath {
	return WeightedPath{Weight: w, Next: n}
}

func NewWeightedBranch(w float64, branches []WeightedPath) WeightedPath {
	var total float64
	for _, b := range branches {
		total += b.Weight
	}
	return WeightedPath{
		Weight: w,
		Next: func(ctx context.Context, sp au.WorkspaceProvider) (int, error) {
			// normalize into total range (eg: 4+3+2+6+1 = 16) => 9
			n := rand.Float64() * total
			for _, b := range branches {
				// loop through, if we are within the contribution factor, then we matched this branch
				if n <= b.Weight {
					return b.Next(ctx, sp)
				}
				// otherwise subtract the weight and go to the next one
				n -= b.Weight
			}
			return branches[len(branches)-1].Next(ctx, sp)
		},
	}
}

func pickRandomMatching[k any](items []k, filter func(k) bool) *k {
	rand.Shuffle(len(items), func(i, j int) {
		a, b := items[i], items[j]
		items[i] = b
		items[j] = a
	})
	for _, i := range items {
		if filter(i) {
			return &i
		}
	}
	return nil
}

var fakeDataGenerators = NewWeightedBranch(1, []WeightedPath{
	// 60% of the time we add todos or comments
	NewWeightedBranch(60, []WeightedPath{
		// 50% of the time we create todos
		NewWeightedBranch(50, []WeightedPath{
			// 50% of the time we create just with a title
			NewWeightedPath(50, func(ctx context.Context, sp au.WorkspaceProvider) (int, error) {
				var params au.CreateTodoParams
				params.CreatedBy = "Fake Author <name@host.com>"
				params.Title = randomTitle()
				_, err := sp.CreateTodo(ctx, params)
				return 1, err
			}),
			// 50% of the time we also have a description
			NewWeightedPath(50, func(ctx context.Context, sp au.WorkspaceProvider) (int, error) {
				var params au.CreateTodoParams
				params.CreatedBy = "Fake Author <name@host.com>"
				params.Title = randomTitle()
				params.Description = randomContent()
				_, err := sp.CreateTodo(ctx, params)
				return 1, err
			}),
		}),
		// 50% of the time we add a comment somewhere
		NewWeightedPath(50, func(ctx context.Context, sp au.WorkspaceProvider) (int, error) {
			if todos, err := sp.ListTodos(ctx); err != nil {
				return 0, err
			} else if picked := pickRandomMatching(todos, func(t au.Todo) bool {
				return t.Status == "open"
			}); picked != nil {
				var params au.CreateCommentParams
				params.CreatedBy = picked.CreatedBy
				params.MediaType = "text/markdown"
				params.Content = []byte(randomContent())
				_, err := sp.CreateComment(ctx, picked.Id, params)
				return 1, err
			}
			return 0, nil
		}),
	}),
	// 30% of the time we modify todos
	NewWeightedBranch(30, []WeightedPath{
		NewWeightedPath(20, func(ctx context.Context, sp au.WorkspaceProvider) (int, error) {
			if todos, err := sp.ListTodos(ctx); err != nil {
				return 0, err
			} else if picked := pickRandomMatching(todos, func(t au.Todo) bool {
				return t.Status == "open"
			}); picked != nil {
				_, err := sp.EditTodo(ctx, picked.Id, au.EditTodoParams{UpdatedBy: picked.CreatedBy, Annotations: map[string]string{
					"https://aurelian.one/annotations/rank": strconv.Itoa(rand.Intn(20)),
				}})
				return 1, err
			}
			return 0, nil
		}),
	}),
	// 10% of the time we delete todos or comments
	NewWeightedBranch(10, []WeightedPath{
		NewWeightedPath(50, func(ctx context.Context, sp au.WorkspaceProvider) (int, error) {
			if todos, err := sp.ListTodos(ctx); err != nil {
				return 0, err
			} else if picked := pickRandomMatching(todos, func(t au.Todo) bool {
				return t.Status == "closed"
			}); picked != nil {
				return 1, sp.DeleteTodo(ctx, picked.Id, au.DeleteTodoParams{DeletedBy: picked.CreatedBy})
			}
			return 0, nil
		}),
		NewWeightedPath(50, func(ctx context.Context, sp au.WorkspaceProvider) (int, error) {
			if todos, err := sp.ListTodos(ctx); err != nil {
				return 0, err
			} else if picked := pickRandomMatching(todos, func(t au.Todo) bool {
				return t.CommentCount > 0
			}); picked != nil {
				if comments, err := sp.ListComments(ctx, picked.Id); err != nil {
					return 0, err
				} else if pickedComment := pickRandomMatching(comments, func(c au.Comment) bool { return true }); pickedComment != nil {
					return 1, sp.DeleteComment(ctx, picked.Id, pickedComment.Id, au.DeleteCommentParams{DeletedBy: picked.CreatedBy})
				}
			}
			return 0, nil
		}),
	}),
})

var sentences = []string{"360 degree content marketing pool green technology and climate change , so big picture.",
	"4-blocker new economy cc me on that cloud native container based time to open the kimono.",
	"A loss a day will keep you focus cannibalize, nor quick sync high touch client, or wiggle room onward and upward, productize the deliverables and focus on the bottom line.",
	"A loss a day will keep you focus up the flagpole bazooka that run it past the boss jump right in and banzai attack will they won't they its all greek to me unless they bother until the end of time maybe vis a vis too many cooks over the line dear hiring manager:.",
	"A tentative event rundown is attached for your reference, including other happenings on the day you are most welcome to join us beforehand for a light lunch we would also like to invite you to other activities on the day, including the interim and closing panel discussions on the intersection of businesses and social innovation, and on building a stronger social innovation eco-system respectively bottleneck mice hit the ground running if you could do that, that would be great, or turd polishing, for we've bootstrapped the model highlights.",
	"After I ran into Helen at a restaurant, I realized she was just office pretty low engagement out of scope.",
	"Agile are we in agreeance, and run it up the flagpole, ping the boss and circle back, closer to the metal what the.",
	"Are there any leftovers in the kitchen?.",
	"Are we in agreeance.",
	"Back-end of third quarter this is our north star design i dont care if you got some copy, why you dont use officeipsumcom or something like that ?.",
	"Best practices low hanging fruit, for high performance keywords, or not a hill to die on feed the algorithm crank this out.",
	"Big data this is our north star design on-brand but completeley fresh personal development, yet feed the algorithm.",
	"Blue money identify pain points, and run it up the flagpole, ping the boss and circle back, or call in the air support, for deliverables.",
	"Both the angel on my left shoulder and the devil on my right are eager to go to the next board meeting and say weâ€™re ditching the business model hammer out, and exposing new ways to evolve our design language, but downselect, nor make it more corporate please.",
	"Bottleneck mice.",
	"Cadence hammer out incentivization pull in ten extra bodies to help roll the tortoise circle back.",
	"Can we align on lunch orders value-added.",
	"Can you champion this feature creep tread it daily deploy to production, but we need to build it so that it scales.",
	"Can you run this by clearance? hot johnny coming through.",
	"Canatics exploratory investigation data masking it's about managing expectations, for creativity requires you to murder your children, so roll back strategy exposing new ways to evolve our design language cannibalize groom the backlog.",
	"Canatics exploratory investigation data masking we need evergreen content.",
	"Cc me on that we need to start advertising on social media cc me on that.",
	"Churning anomalies granularity, and how much bandwidth do you have introduccion.",
	"Collaboration through advanced technlogy not a hill to die on in this space.",
	"creativity requires you to murder your children move the needle gain traction, but quantity close the loop.",
	"creativity requires you to murder your children on this journey we have to leverage up the messaging knowledge is power, for going forward sea change, and parallel path.",
	"Crisp ppt reach out, but timeframe, for curate, or product launch, or drink the Kool-aid.",
	"Cross functional teams enable out of the box brainstorming.",
	"Cross pollination across our domains my supervisor didn't like the latest revision you gave me can you switch back to the first revision?, and tiger team, and overcome key issues to meet key milestones turn the ship increase the pipelines.",
	"Cross sabers on your plate, nor feed the algorithm, or that ipo will be a game-changer, yet problem territories cloud strategy.",
	"Dear hiring manager: keep it lean.",
	"Design thinking table the discussion , big boy pants, and that's not on the roadmap prairie dogging.",
	"Digitalize.",
	"Disband the squad but rehydrate as needed.",
	"Downselect granularity, nor win-win-win make it a priority.",
	"Downselect message the initiative.",
	"Draw a line in the sand that's mint, well done.",
	"Everyone thinks the soup tastes better after theyâ€™ve pissed in it if you could do that, that would be great, yet touch base effort made was a lot.",
	"Everyone thinks the soup tastes better after theyâ€™ve pissed in it wheelhouse, yet turn the crank, Q1, nor drop-dead date customer centric.",
	"Fire up your browser high performance keywords, so dunder mifflin, so when does this sunset?.",
	"First-order optimal strategies how much bandwidth do you have, can we jump on a zoom, for tbrand terrorists, so land it in region.",
	"Flesh that out can I just chime in on that one moving the goalposts, so meeting assassin, so single wringable neck where the metal hits the meat design thinking.",
	"Forcing function first-order optimal strategies run it up the flag pole canatics exploratory investigation data masking, or feed the algorithm we need to touch base off-line before we fire the new ux experience, and punter.",
	"Gain traction cross-pollination feed the algorithm, for let's put a pin in that, or highlights.",
	"Gain traction disband the squad but rehydrate as needed win-win-win bottleneck mice, or feed the algorithm.",
	"Gain traction pre launch, pixel pushing.",
	"Get all your ducks in a row run it up the flag pole slipstream, so regroup, so good optics, nor scope creep.",
	"Get in the driver's seat hit the ground running low-hanging fruit incentivization, and flesh that out value prop, for collaboration through advanced technlogy.",
	"Globalize that's mint, well done tbrand terrorists 4-blocker, or action item, or guerrilla marketing we need to follow protocol.",
	"Goalposts our competitors are jumping the shark, or eat our own dog food.",
	"Going forward tread it daily.",
	"Golden goose move the needle, but sorry i didn't get your email, yet spinning our wheels, nor per my previous email.",
	"Great plan! let me diarize this, and we can synchronise ourselves at a later timepoint tribal knowledge, for finance everyone thinks the soup tastes better after theyâ€™ve pissed in it, nor not enough bandwidth quick sync nobody's fault it could have been managed better.",
	"Have bandwidth.",
	"Hire the best we need evergreen content.",
	"I dont care if you got some copy, why you dont use officeipsumcom or something like that ? circle back, nor we need to get all stakeholders up to speed and in the right place market-facing on-brand but completeley fresh, and synergestic actionables performance review.",
	"I dont care if you got some copy, why you dont use officeipsumcom or something like that ? synergestic actionables, but cannibalize, draft policy ppml proposal move the needle crisp ppt, so beef up.",
	"I know you're busy circle back, for deploy to production.",
	"I need to pee and then go to another meeting powerPointless, or sacred cow, nor prethink.",
	"I need to pee and then go to another meeting.",
	"I'll book a meeting so we can solution this before the sprint is over minimize backwards overflow locked and loaded, so helicopter view drink the Kool-aid.",
	"I'm sorry i replied to your emails after only three weeks, but can the site go live tomorrow anyway? downselect let's circle back to that this is not the hill i want to die on low-hanging fruit synergize productive mindfulness fire up your browser.",
	"If you're not hurting you're not winning encourage & support business growth , so due diligence pushback on this journey, or this is not a video game, this is a meeting! to be inspired is to become creative, innovative and energized we want this philosophy to trickle down to all our stakeholders.",
	"In this space.",
	"Increase the pipelines.",
	"Innovation is hot right now a better understanding of usage can aid in prioritizing future efforts this medium needs to be more dynamic.",
	"Innovation is hot right now race without a finish line let's not solutionize this right now parking lot it build on a culture of contribution and inclusion.",
	"Into the weeds it is all exactly as i said, but i don't like it.",
	"It's a simple lift and shift job we need to follow protocol, or 360 degree content marketing pool work flows in this space, and quick-win, nor we should leverage existing asserts that ladder up to the message.",
	"Ladder up / ladder back to the strategy five-year strategic plan granularity, and baseline.",
	"Lean into that problem manage expectations we need to think big start small and scale fast to energize our clients product market fit, big boy pants.",
	"Let's see if we can dovetail these two projects win-win let's put a pin in that incentivization baseline, yet pulling teeth.",
	"Let's take this conversation offline we want to empower the team with the right tools and guidance to uplevel our craft and build better, or deploy to production, but cloud native container based weâ€™re starting to formalize flexible opinions around our foundations, or beef up.",
	"Let's unpack that later.",
	"Lift and shift message the initiative, or customer centric market-facing, for closer to the metal let's unpack that later.",
	"Locked and loaded nail jelly to the hothouse wall.",
	"Looks great, can we try it a different way i also believe it's important for every member to be involved and invested in our company and this is one way to do so, or helicopter view, so locked and loaded.",
	"Loop back out of the loop.",
	"Low hanging fruit business impact.",
	"Low-hanging fruit.",
	"Make it look like digital UI groom the backlog, and agile, or throughput loop back.",
	"Make sure to include in your wheelhouse what's the status on the deliverables for eow?.",
	"Manage expectations high performance keywords, yet hammer out note for the previous submit: the devil should be on the left shoulder, but c-suite pull in ten extra bodies to help roll the tortoise nobody's fault it could have been managed better.",
	"Manage expectations we need to dialog around your choice of work attire, nor prethink, nor drink from the firehose, yet not the long pole in my tent.",
	"Market-facing.",
	"Marketing computer development html roi feedback team website into the weeds synergize productive mindfulness in an ideal world.",
	"Message the initiative out of scope, nor pixel pushing.",
	"Moving the goalposts paddle on both sides optics, so lift and shift.",
	"Nail jelly to the hothouse wall work punter close the loop, nor we need to harvest synergy effects.",
	"Nobody's fault it could have been managed better i also believe it's important for every member to be involved and invested in our company and this is one way to do so.",
	"Old boys club mumbo jumbo, nor weaponize the data, so we need more paper, nor we need a recap by eod, cob or whatever comes first.",
	"On this journey.",
	"On your plate performance review turn the crank, and weâ€™re all in this together, even if our businesses function differently, or my capacity is full we need to dialog around your choice of work attire turn the ship.",
	"Onward and upward, productize the deliverables and focus on the bottom line dunder mifflin, yet circle back around, nor wheelhouse we need to harvest synergy effects note for the previous submit: the devil should be on the left shoulder, but we can't hear you.",
	"Optics if you want to motivate these clowns, try less carrot and more stick, or drill down, but quick sync, but deploy.",
	"Optics incentivize adoption timeframe deep dive.",
	"Optimize for search let's see if we can dovetail these two projects organic growth.",
	"Organic growth.",
	"Paddle on both sides at the end of the day, but increase the resolution, scale it up we need a larger print, or slipstream throughput.",
	"Paddle on both sides up the flagpole bazooka that run it past the boss jump right in and banzai attack will they won't they its all greek to me unless they bother until the end of time maybe vis a vis too many cooks over the line, but what are the expectations, but marginalised key performance indicators focus on the customer journey, zeitgeist.",
	"Pig in a python i don't want to drain the whole swamp, i just want to shoot some alligators, and service as core &innovations as power makes our brand, for green technology and climate change if you're not hurting you're not winning, for gain traction.",
	"Pig in a python move the needle post launch game plan knowledge is power what the business impact.",
	"Pixel pushing close the loop globalize hard stop, for prepare yourself to swim with the sharks ping me I just wanted to give you a heads-up.",
	"Please advise soonest pivot, or we need to socialize the comms with the wider stakeholder community.",
	"Please use \"solutionise\" instead of solution ideas! :) let's circle back tomorrow.",
	"Please use \"solutionise\" instead of solution ideas! :) player-coach.",
	"Poop data-point.",
	"PowerPointless create spaces to explore whatâ€™s next, or proceduralize.",
	"Price point knowledge is power gain alignment we need to socialize the comms with the wider stakeholder community let's prioritize the low-hanging fruit, for performance review.",
	"Price point we should have a meeting to discuss the details of the next meeting let's pressure test this, but value prop, for dunder mifflin.",
	"Product launch let's circle back tomorrow, but flesh that out.",
	"Productize donuts in the break room, and streamline, yet we need to have a Come to Jesus meeting with Phil about his attitude.",
	"Productize we're building the plane while we're flying it create spaces to explore whatâ€™s next diversify kpis, imagineer.",
	"Productize.",
	"Programmatically circle back around, can you run this by clearance? hot johnny coming through , for products need full resourcing and support from a cross-functional team in order to be built, maintained, and evolved make sure to include in your wheelhouse.",
	"Pulling teeth this medium needs to be more dynamic, for run it up the flagpole canatics exploratory investigation data masking.",
	"Put it on the parking lot touch base we need to make the new version clean and sexy.",
	"Put your feelers out zeitgeist prepare yourself to swim with the sharks identify pain points.",
	"Quarterly sales are at an all-time low can I just chime in on that one core competencies.",
	"Quick win creativity requires you to murder your children player-coach draw a line in the sand looks great, can we try it a different way marketing, illustration re-inventing the wheel.",
	"Quick-win pig in a python.",
	"Radical candor blue sky, so Q1, nor 360 degree content marketing pool granularity five-year strategic plan, nor optimize for search.",
	"Ramp up we need to start advertising on social media, yet run it up the flag pole we need to button up our approach, nor run it up the flag pole.",
	"Red flag can I just chime in on that one run it up the flag pole, or it is all exactly as i said, but i don't like it.",
	"Roll back strategy cross sabers lift and shift but what's the real problem we're trying to solve here?, but don't over think it, yet bells and whistles.",
	"Run it up the flag pole we need to build it so that it scales, or unlock meaningful moments of relaxation punter, and Q1, so incentivize adoption 60% to 30% is a lot of persent.",
	"Scope creep synergestic actionables, or reinvent the wheel, for we want to empower the team with the right tools and guidance to uplevel our craft and build better, and can you send me an invite?.",
	"Shotgun approach it's not hard guys, nor put your feelers out, but low hanging fruit, or loop back, so ultimate measure of success, yet game plan.",
	"Show pony low engagement we don't want to boil the ocean optimize for search race without a finish line, so up the flagpole bazooka that run it past the boss jump right in and banzai attack will they won't they its all greek to me unless they bother until the end of time maybe vis a vis too many cooks over the line, or time to open the kimono.",
	"Slipstream I have zero cycles for this.",
	"Sorry i was triple muted data-point, face time, so great plan! let me diarize this, and we can synchronise ourselves at a later timepoint, and tribal knowledge parallel path clear blue water.",
	"Start procrastinating 2 hours get to do work while procrastinating open book pretend to read while manager stands and watches silently nobody is looking quick do your web search manager caught you and you are fured we need to touch base off-line before we fire the new ux experience, we want to see more charts both the angel on my left shoulder and the devil on my right are eager to go to the next board meeting and say weâ€™re ditching the business model cta close the loop.",
	"Strategic staircase they have downloaded gmail and seems to be working for now, nor good optics, nor killing it, or turn the crank, nor sacred cow, and bench mark.",
	"Table the discussion groom the backlog, yet screw the pooch hop on the bandwagon tread it daily, nor identify pain points.",
	"Table the discussion low hanging fruit.",
	"Take five, punch the tree, and come back in here with a clear head ultimate measure of success cross-pollination slipstream, so product management breakout fastworks.",
	"Technologically savvy can we take this offline, but old boys club one-sheet onward and upward, productize the deliverables and focus on the bottom line.",
	"That is a good problem to have.",
	"That jerk from finance really threw me under the bus business impact, yet make it a priority call in the air support, so business impact.",
	"Thinking outside the box thought shower can we jump on a zoom, yet in this space poop push back powerPointless.",
	"This is our north star design one-sheet thought shower, we need to future-proof this.",
	"This medium needs to be more dynamic.",
	"This vendor is incompetent moving the goalposts, yet viral engagement, for roll back strategy make it more corporate please.",
	"Thought shower.",
	"Three-martini lunch obviously helicopter view.",
	"Three-martini lunch onward and upward, productize the deliverables and focus on the bottom line open door policy, so let's circle back to that.",
	"To be inspired is to become creative, innovative and energized we want this philosophy to trickle down to all our stakeholders let's circle back tomorrow social currency, or radical candor can you run this by clearance? hot johnny coming through , or where the metal hits the meat.",
	"Today shall be a cloudy day, thanks to blue sky thinking, we can now deploy our new ui to the cloud i'm sorry i replied to your emails after only three weeks, but can the site go live tomorrow anyway?.",
	"Today shall be a cloudy day, thanks to blue sky thinking, we can now deploy our new ui to the cloud quick sync, but bells and whistles cross-pollination organic growth.",
	"Translating our vision of having a market leading platfrom hop on the bandwagon, so increase the pipelines.",
	"Tribal knowledge hard stop, downselect, so product market fit, for business impact.",
	"Tribal knowledge make it more corporate please criticality make it look like digital, feature creep, yet not enough bandwidth.",
	"Turn the crank put a record on and see who dances, so can you ballpark the cost per unit for me, canatics exploratory investigation data masking, or optics, for how much bandwidth do you have.",
	"Turn the crank we need distributors to evangelize the new line to local markets cloud native container based i am dead inside.",
	"Turn the ship let's circle back tomorrow, yet can we parallel path window of opportunity take five, punch the tree, and come back in here with a clear head hit the ground running can you put it on my calendar?.",
	"Value-added.",
	"Viral engagement.",
	"Waste of resources work flows , or radical candor can we take this offline, so introduccion Q1.",
	"We are running out of runway great plan! let me diarize this, and we can synchronise ourselves at a later timepoint, but run it up the flag pole, and lose client to 10:00 meeting, and horsehead offer downselect.",
	"We can't hear you deploy we need to touch base off-line before we fire the new ux experience, nor eat our own dog food.",
	"We need a paradigm shift future-proof.",
	"We need a recap by eod, cob or whatever comes first spinning our wheels turd polishing.",
	"We need to build it so that it scales.",
	"We need to crystallize a plan please submit the sop and uat files by next monday, for that jerk from finance really threw me under the bus, or prepare yourself to swim with the sharks show grit, even dead cats bounce cc me on that.",
	"We need to think big start small and scale fast to energize our clients low hanging fruit put in in a deck for our standup today time to open the kimono.",
	"We need to think big start small and scale fast to energize our clients sorry i was triple muted, yet this is not a video game, this is a meeting! value prop.",
	"We should leverage existing asserts that ladder up to the message can we parallel path nobody's fault it could have been managed better, and eat our own dog food, for parallel path parallel path.",
	"We should leverage existing asserts that ladder up to the message curate, but big data.",
	"We should leverage existing asserts that ladder up to the message it is all exactly as i said, but i don't like it.",
	"We want to empower the team with the right tools and guidance to uplevel our craft and build better clear blue water agile we need this overall to be busier and more active, nor they have downloaded gmail and seems to be working for now, yet slow-walk our commitment put a record on and see who dances.",
	"We want to see more charts they have downloaded gmail and seems to be working for now, but this is not the hill i want to die on we want to empower the team with the right tools and guidance to uplevel our craft and build better put it on the parking lot that's mint, well done.",
	"Weâ€™re all in this together, even if our businesses function differently.",
	"What the open door policy what's the status on the deliverables for eow? crisp ppt.",
	"What's our go to market strategy? we need more paper, or face time, but thinking outside the box, for problem territories.",
	"Where do we stand on the latest client ask quick win reach out, so downselect.",
	"Where do we stand on the latest client ask to be inspired is to become creative, innovative and energized we want this philosophy to trickle down to all our stakeholders.",
	"Win-win-win innovation is hot right now, horsehead offer, we just need to put these last issues to bed.",
	"Work flows guerrilla marketing.",
	"Your work on this project has been really impactful this proposal is a win-win situation which will cause a stellar paradigm shift, and produce a multi-fold increase in deliverables prepare yourself to swim with the sharks, but that's not on the roadmap.",
	"Zeitgeist finance, but big data, for quick-win, but I have zero cycles for this we need to build it so that it scales.",
	"We need to socialize the comms with the wider stakeholder community that's not on the roadmap, yet we need to think big start small and scale fast to energize our clients game-plan, so race without a finish line win-win, or we need a recap by eod, cob or whatever comes first.",
}

func randomTitle() string {
	output := strings.TrimSuffix(sentences[rand.Intn(len(sentences))], ".")
	for len(output) > au.MaximumTodoTitleLength {
		si := strings.LastIndex(output, " ")
		output = output[:si]
	}
	return output
}

func randomContent() string {
	paragraphs := 1 + rand.Intn(3)
	output := ""
	for p := 0; p < paragraphs; p += 1 {
		numSentences := 1 + rand.Intn(4)
		for s := 0; s < numSentences; s += 1 {
			output += sentences[rand.Intn(len(sentences))]
		}
		output += "\n"
	}
	for len(output) > au.MaximumDescriptionLength {
		si := strings.LastIndex(output, ".")
		output = output[:si+1]
	}
	return output
}
